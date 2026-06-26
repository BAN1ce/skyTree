package memory

import (
	"sync"
)

// subShard 保存一个分片内的订阅树，并用独立锁减少不同 topic 组之间的读写阻塞。
type subShard struct {
	mux sync.RWMutex
	sub *SubCore
}

// clientShardIndex 记录 clientID 到分片的反向索引，用于按客户端快速清理订阅。
type clientShardIndex struct {
	// 使用计数而不是集合，因为同一个 client 可能在同一个分片内订阅多个 topic。
	mux sync.RWMutex
	m   map[string]map[string]int
}

// newClientShardIndex 创建客户端到分片的反向索引。
func newClientShardIndex() *clientShardIndex {
	return &clientShardIndex{
		m: make(map[string]map[string]int, 1024),
	}
}

// inc 增加一个客户端在指定分片内的订阅计数。
func (i *clientShardIndex) inc(clientID, shardKey string) {
	if clientID == "" || shardKey == "" {
		return
	}
	i.mux.Lock()
	defer i.mux.Unlock()
	if _, ok := i.m[clientID]; !ok {
		i.m[clientID] = make(map[string]int, 4)
	}
	i.m[clientID][shardKey]++
}

// dec 减少一个客户端在指定分片内的订阅计数，并在计数归零时清理索引。
func (i *clientShardIndex) dec(clientID, shardKey string) {
	if clientID == "" || shardKey == "" {
		return
	}
	i.mux.Lock()
	defer i.mux.Unlock()
	shards, ok := i.m[clientID]
	if !ok {
		return
	}
	v, ok := shards[shardKey]
	if !ok {
		return
	}
	v--
	if v <= 0 {
		delete(shards, shardKey)
	} else {
		shards[shardKey] = v
	}
	if len(shards) == 0 {
		delete(i.m, clientID)
	}
}

// shardsForClient 返回客户端可能存在订阅的分片列表。
func (i *clientShardIndex) shardsForClient(clientID string) []string {
	if clientID == "" {
		return nil
	}
	i.mux.RLock()
	defer i.mux.RUnlock()
	shards, ok := i.m[clientID]
	if !ok || len(shards) == 0 {
		return nil
	}
	out := make([]string, 0, len(shards))
	for k := range shards {
		out = append(out, k)
	}
	return out
}

// deleteClient 删除客户端的所有分片索引。
func (i *clientShardIndex) deleteClient(clientID string) {
	if clientID == "" {
		return
	}
	i.mux.Lock()
	defer i.mux.Unlock()
	delete(i.m, clientID)
}

// MemorySubCenter 是订阅中心的内存实现，主要供状态机、WAL 和测试复用。
type MemorySubCenter struct {
	// 每个分片都有独立 RWMutex；无关 topic 组的读写可以并发执行。
	shardMux sync.RWMutex
	shards   map[string]*subShard
	index    *clientShardIndex

	// clientOwnerTokens 保存每个 clientID 当前有效的 fencing token。
	ownerMux          sync.RWMutex
	clientOwnerTokens map[string]string
}

// NewMemorySubCenter 创建空的内存订阅中心。
func NewMemorySubCenter() *MemorySubCenter {
	return &MemorySubCenter{
		shards:            make(map[string]*subShard, 128),
		index:             newClientShardIndex(),
		clientOwnerTokens: make(map[string]string, 1024),
	}
}
