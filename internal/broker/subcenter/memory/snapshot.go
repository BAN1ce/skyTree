package memory

import (
	"io"
	"sort"

	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
	"google.golang.org/protobuf/proto"
)

// WriteSnapshot 将完整内存订阅状态序列化到 writer。
// 导出该方法是为了让状态机层委托快照逻辑，而不直接访问私有字段。
func (t *MemorySubCenter) WriteSnapshot(writer io.Writer) error {
	t.shardMux.RLock()
	keys := make([]string, 0, len(t.shards))
	for k := range t.shards {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Marshal 期间持有所有分片读锁，避免 map 和 Trie 结构被并发修改。
	for _, k := range keys {
		t.shards[k].mux.RLock()
	}
	defer func() {
		for i := len(keys) - 1; i >= 0; i-- {
			t.shards[keys[i]].mux.RUnlock()
		}
		t.shardMux.RUnlock()
	}()

	snapshot := &proto2.ClusterSub{
		Cluster:           make(map[string]*proto2.SubCore, len(keys)),
		ClientOwnerTokens: t.copyOwnerTokens(),
	}
	for _, k := range keys {
		snapshot.Cluster[k] = t.shards[k].sub.core
	}
	data, err := proto.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// RecoverSnapshot 从快照 reader 恢复内存订阅状态。
func (t *MemorySubCenter) RecoverSnapshot(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	clusterSub := new(proto2.ClusterSub)
	if err := proto.Unmarshal(data, clusterSub); err != nil {
		return err
	}

	newShards := make(map[string]*subShard, len(clusterSub.GetCluster()))
	for k, v := range clusterSub.GetCluster() {
		newShards[k] = &subShard{sub: NewSubCoreFromProto(v)}
	}
	t.shardMux.Lock()
	t.shards = newShards
	t.shardMux.Unlock()

	// 从快照恢复 owner token；旧快照没有该字段时重新初始化空 map。
	t.ownerMux.Lock()
	if clusterSub.GetClientOwnerTokens() != nil {
		t.clientOwnerTokens = make(map[string]string, len(clusterSub.GetClientOwnerTokens()))
		for k, v := range clusterSub.GetClientOwnerTokens() {
			t.clientOwnerTokens[k] = v
		}
	} else {
		t.clientOwnerTokens = make(map[string]string, 1024)
	}
	t.ownerMux.Unlock()

	t.rebuildClientIndexFromShards()
	return nil
}
