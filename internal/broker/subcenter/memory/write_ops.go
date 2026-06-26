package memory

import (
	"context"
	"errors"

	"github.com/BAN1ce/skyTree/logger"
	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// CreateSub 创建或更新客户端订阅，并维护客户端到分片的反向索引。
func (t *MemorySubCenter) CreateSub(ctx context.Context, req *proto2.SubRequest) (*proto2.SubResponse, error) {
	_ = ctx
	response := &proto2.SubResponse{
		Topics: make(map[string]int32),
	}
	var errs error

	// Fencing：确保订阅请求来自当前连接 owner。
	if req.GetOwnerToken() != "" {
		stored := t.getOwnerToken(req.GetClientID())
		if stored == "" {
			// 重启后或显式设置 owner token 前的首次写入，接受并绑定该 owner。
			t.setOwnerToken(req.GetClientID(), req.GetOwnerToken())
		} else if stored != req.GetOwnerToken() {
			// 预期内的 fencing 失败不能作为 Raft apply 错误返回，只通过 ack=-1 表达失败。
			for _, subOption := range req.GetTopics() {
				topic := subOption.GetTopic()
				if topic == "" {
					continue
				}
				response.Topics[topic] = -1
			}
			response.Success = false
			return response, nil
		}
	}

	for _, subOption := range req.Topics {
		topic := subOption.GetTopic()
		shardKey := shardKeyFromTopic(topic)
		shard := t.getOrCreateShard(shardKey)

		shard.mux.Lock()
		already := false
		if subTopics, ok := shard.sub.clients.Clients[req.ClientID]; ok {
			if _, exists := subTopics.Topics[topic]; exists {
				already = true
			}
		}

		err := shard.sub.createSub(req.ClientID, subOption)
		shard.mux.Unlock()

		if err != nil {
			logger.Logger.Error().Err(err).Msg("handle normal sub request error")
			errs = errors.Join(errs, err)
			response.Topics[topic] = -1
			continue
		}
		if !already {
			t.index.inc(req.ClientID, shardKey)
		}
		response.Topics[topic] = subOption.GetQoS()
	}

	return response, errs
}

// DeleteSub 删除客户端订阅，并按 MQTT 语义返回每个 topic 的取消订阅结果码。
func (t *MemorySubCenter) DeleteSub(ctx context.Context, req *proto2.UnSubRequest) (*proto2.UnSubResponse, error) {
	_ = ctx
	response := &proto2.UnSubResponse{}

	// Fencing：确保取消订阅请求来自当前连接 owner。
	if req.GetOwnerToken() != "" {
		if stored := t.getOwnerToken(req.GetClientID()); stored != req.GetOwnerToken() {
			// 与 CreateSub 一致，owner 不匹配只体现在业务响应中，不作为 Raft apply 错误。
			for range req.GetTopics() {
				response.Topics = append(response.Topics, -1)
			}
			return response, nil
		}
	}

	for _, topic := range req.Topics {
		shardKey := shardKeyFromTopic(topic)
		shard, ok := t.getShard(shardKey)
		if !ok {
			response.Topics = append(response.Topics, 0x11)
			continue
		}
		shard.mux.Lock()
		existed := false
		if subTopics, ok := shard.sub.clients.Clients[req.ClientID]; ok {
			if _, ok2 := subTopics.Topics[topic]; ok2 {
				existed = true
			}
		}
		shard.sub.deleteSub(&proto2.SubOption{Topic: topic}, req.ClientID)
		shard.mux.Unlock()
		if existed {
			t.index.dec(req.ClientID, shardKey)
			response.Topics = append(response.Topics, 0)
		} else {
			response.Topics = append(response.Topics, 0x11)
		}
	}

	return response, nil
}

// DeleteTopic 删除指定 topic filter 下的所有订阅。
func (t *MemorySubCenter) DeleteTopic(ctx context.Context, req *proto2.DeleteTopicRequest) (*proto2.DeleteTopicResponse, error) {
	_ = ctx
	response := &proto2.DeleteTopicResponse{Success: true}

	shardKey := shardKeyFromTopic(req.Topic)
	shard, ok := t.getShard(shardKey)
	if !ok {
		return response, nil
	}
	shard.mux.Lock()
	removedClientIDs := shard.sub.deleteTopic(req.Topic)
	shard.mux.Unlock()
	for _, clientID := range removedClientIDs {
		t.index.dec(clientID, shardKey)
	}
	return response, nil
}

// DeleteClient 删除客户端在所有分片中的订阅，owner token 存在时会执行 fencing 校验。
func (t *MemorySubCenter) DeleteClient(ctx context.Context, req *proto2.DeleteClientRequest) (*proto2.DeleteClientResponse, error) {
	_ = ctx
	// OwnerToken 存在时只允许当前 owner 删除；空 token 表示管理面或兼容路径的强制删除。
	if req.GetOwnerToken() != "" {
		if stored := t.getOwnerToken(req.GetClientID()); stored != req.GetOwnerToken() {
			return &proto2.DeleteClientResponse{Success: false}, nil
		}
	}
	shardKeys := t.index.shardsForClient(req.ClientID)
	if len(shardKeys) == 0 {
		return &proto2.DeleteClientResponse{Success: true}, nil
	}

	for _, shardKey := range shardKeys {
		shard, ok := t.getShard(shardKey)
		if !ok {
			continue
		}
		shard.mux.Lock()
		shard.sub.deleteClient(req.ClientID)
		shard.mux.Unlock()
	}
	// 所有分片清理完成后再删除反向索引。
	t.index.deleteClient(req.ClientID)

	return &proto2.DeleteClientResponse{Success: true}, nil
}

// SetClientOwnerToken 设置客户端当前 owner token，用于后续订阅写入的 fencing 校验。
func (t *MemorySubCenter) SetClientOwnerToken(ctx context.Context, req *proto2.SetClientOwnerTokenRequest) (*proto2.SetClientOwnerTokenResponse, error) {
	_ = ctx
	if req.GetClientID() == "" {
		return &proto2.SetClientOwnerTokenResponse{Success: false}, nil
	}
	t.setOwnerToken(req.GetClientID(), req.GetOwnerToken())
	return &proto2.SetClientOwnerTokenResponse{Success: true}, nil
}
