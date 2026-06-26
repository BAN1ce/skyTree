package memory

import (
	"context"

	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// GetClientSubscriptions 返回客户端当前保存的所有订阅副本。
func (t *MemorySubCenter) GetClientSubscriptions(ctx context.Context, req *proto2.GetClientSubscriptionsRequest) (*proto2.GetClientSubscriptionsResponse, error) {
	_ = ctx
	response := &proto2.GetClientSubscriptionsResponse{
		Topics: make(map[string]*proto2.SubOption),
	}

	clientID := req.GetClientID()
	if clientID == "" {
		return response, nil
	}

	// 遍历所有分片查找该客户端订阅，返回副本避免调用方修改内部状态。
	t.shardMux.RLock()
	defer t.shardMux.RUnlock()

	for _, shard := range t.shards {
		shard.mux.RLock()
		if subTopics, ok := shard.sub.clients.Clients[clientID]; ok && subTopics != nil {
			for topic, subOption := range subTopics.Topics {
				if subOption != nil {
					response.Topics[topic] = &proto2.SubOption{
						Topic:                  subOption.GetTopic(),
						QoS:                    subOption.GetQoS(),
						RetainAsPublished:      subOption.GetRetainAsPublished(),
						NoLocal:                subOption.GetNoLocal(),
						RetainHandling:         subOption.GetRetainHandling(),
						SubscriptionIdentifier: subOption.GetSubscriptionIdentifier(),
					}
				}
			}
		}
		shard.mux.RUnlock()
	}

	return response, nil
}
