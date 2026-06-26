package memory

import topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"

// getShard 读取指定分片，不存在时返回 false。
func (t *MemorySubCenter) getShard(key string) (*subShard, bool) {
	t.shardMux.RLock()
	defer t.shardMux.RUnlock()
	shard, ok := t.shards[key]
	return shard, ok
}

// getOrCreateShard 获取或创建指定分片，使用双重检查减少写锁竞争。
func (t *MemorySubCenter) getOrCreateShard(key string) *subShard {
	if key == "" {
		key = "_default"
	}

	t.shardMux.RLock()
	shard, ok := t.shards[key]
	t.shardMux.RUnlock()
	if ok {
		return shard
	}

	t.shardMux.Lock()
	defer t.shardMux.Unlock()
	if shard, ok = t.shards[key]; ok {
		return shard
	}
	shard = &subShard{sub: NewSubCore()}
	t.shards[key] = shard
	return shard
}

// shardKeyFromTopic 根据 topic 的第一个非空层级选择分片。
func shardKeyFromTopic(topic string) string {
	// 对 "/a/b"，SplitTopicLevels 返回 ["","a","b"]，分片 key 为 "a"。
	levels := topicutil.SplitTopicLevels(topic)
	if len(levels) == 0 {
		return "_default"
	}
	for _, lv := range levels {
		if lv == "" {
			continue
		}
		return lv
	}
	return "_default"
}

// shardKeysForMatch 返回发布 topic 可能命中的分片集合。
func shardKeysForMatch(publishTopic string) []string {
	// 需要检查：自身首层分片、"+" 首层通配分片、"#" 全局分片，以及共享订阅所在的 "$share" 分片。
	key := shardKeyFromTopic(publishTopic)
	if key == "+" || key == "#" {
		return []string{key}
	}
	if key == "$share" {
		return []string{"$share", "+", "#"}
	}
	return []string{key, "+", "#", "$share"}
}

// rebuildClientIndexFromShards 从所有分片重建客户端反向索引，主要用于快照恢复后。
func (t *MemorySubCenter) rebuildClientIndexFromShards() {
	newIdx := newClientShardIndex()

	t.shardMux.RLock()
	for shardKey, shard := range t.shards {
		shard.mux.RLock()
		for clientID, subTopics := range shard.sub.clients.Clients {
			if subTopics == nil {
				continue
			}
			cnt := len(subTopics.Topics)
			if cnt <= 0 {
				continue
			}
			newIdx.mux.Lock()
			if _, ok := newIdx.m[clientID]; !ok {
				newIdx.m[clientID] = make(map[string]int, 4)
			}
			newIdx.m[clientID][shardKey] += cnt
			newIdx.mux.Unlock()
		}
		shard.mux.RUnlock()
	}
	t.shardMux.RUnlock()

	t.index = newIdx
}
