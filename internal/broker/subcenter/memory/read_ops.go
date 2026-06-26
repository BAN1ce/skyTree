package memory

import (
	"context"

	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// GetAllMatchTopics 返回能匹配发布 topic 的订阅过滤器及最大 QoS。
func (t *MemorySubCenter) GetAllMatchTopics(ctx context.Context, req *proto2.GetAllMatchTopicsRequest) (*proto2.GetAllMatchTopicsResponse, error) {
	_ = ctx
	topic := req.Topic
	response := &proto2.GetAllMatchTopicsResponse{
		Topic: make(map[string]int32),
	}

	for _, shardKey := range shardKeysForMatch(topic) {
		shard, ok := t.getShard(shardKey)
		if !ok {
			continue
		}
		shard.mux.RLock()
		matchTopic := shard.sub.matchTopic(topic)
		shard.mux.RUnlock()

		for subTopic, maxQoS := range matchTopic {
			if old, exists := response.Topic[subTopic]; !exists || maxQoS > old {
				response.Topic[subTopic] = maxQoS
			}
		}
	}

	return response, nil
}

// GetAllMatchClient 返回匹配发布 topic 的客户端，并为每个客户端折叠出最优订阅参数。
func (t *MemorySubCenter) GetAllMatchClient(ctx context.Context, req *proto2.GetAllSubTopicClientRequest) (*proto2.GetAllSubTopicClientResponse, error) {
	_ = ctx
	topic := req.Topic
	response := &proto2.GetAllSubTopicClientResponse{
		Client: make(map[string]*proto2.SubOption),
	}

	for _, shardKey := range shardKeysForMatch(topic) {
		shard, ok := t.getShard(shardKey)
		if !ok {
			continue
		}
		shard.mux.RLock()
		matchClient := shard.sub.matchTopicClient(topic)
		shard.mux.RUnlock()

		for clientID, subOption := range matchClient {
			if old, exists := response.Client[clientID]; !exists || subOption.GetQoS() > old.GetQoS() {
				response.Client[clientID] = subOption
			}
		}
	}

	return response, nil
}

// GetAllMatchClientV2 返回每个客户端的完整命中订阅列表，不折叠重叠订阅。
func (t *MemorySubCenter) GetAllMatchClientV2(ctx context.Context, req *proto2.GetAllMatchClientV2Request) (*proto2.GetAllMatchClientV2Response, error) {
	_ = ctx
	topic := req.GetTopic()

	// clientID -> 聚合后的匹配结果。
	type agg struct {
		maxQoS  int32
		matched []*proto2.MatchedSubscription
	}
	aggMap := make(map[string]*agg, 1024)

	for _, shardKey := range shardKeysForMatch(topic) {
		shard, ok := t.getShard(shardKey)
		if !ok {
			continue
		}
		shard.mux.RLock()
		matchClient := shard.sub.matchTopicClientV2(topic)
		shard.mux.RUnlock()

		for clientID, subs := range matchClient {
			if len(subs) == 0 {
				continue
			}
			a, ok := aggMap[clientID]
			if !ok {
				a = &agg{}
				aggMap[clientID] = a
			}
			for _, ms := range subs {
				if ms == nil {
					continue
				}
				if ms.GetQoS() > a.maxQoS {
					a.maxQoS = ms.GetQoS()
				}
				a.matched = append(a.matched, ms)
			}
		}
	}

	resp := &proto2.GetAllMatchClientV2Response{Matches: make([]*proto2.ClientMatch, 0, len(aggMap))}
	for clientID, a := range aggMap {
		if a == nil {
			continue
		}
		resp.Matches = append(resp.Matches, &proto2.ClientMatch{
			ClientID: clientID,
			MaxQoS:   a.maxQoS,
			Matched:  a.matched,
		})
	}
	return resp, nil
}

// GetAllMatchTopicsForWildTopic 返回能被指定通配过滤器覆盖的已订阅 topic filter。
func (t *MemorySubCenter) GetAllMatchTopicsForWildTopic(ctx context.Context, req *proto2.GetAllMatchTopicsForWildTopicRequest) (*proto2.GetAllMatchTopicsForWildTopicResponse, error) {
	_ = ctx
	response := &proto2.GetAllMatchTopicsForWildTopicResponse{}

	var topics []string
	t.shardMux.RLock()
	for _, shard := range t.shards {
		shard.mux.RLock()
		topics = append(topics, shard.sub.matchTopicForWildcard(req.Topic)...)
		shard.mux.RUnlock()
	}
	t.shardMux.RUnlock()

	response.Topic = topics
	return response, nil
}
