package raft

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/broker/subcenter/statemachine"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// Cluster 是 subscription.Center 的 Raft 客户端代理实现。
type Cluster struct {
	cluster cluster.Client
}

// NewCluster 创建 Raft 订阅中心代理。
func NewCluster(cluster cluster.Client) *Cluster {
	return &Cluster{
		cluster: cluster,
	}
}

// CreateSub 通过 Raft 写入创建或更新订阅。
func (c *Cluster) CreateSub(ctx context.Context, option *proto.SubRequest) (*proto.SubResponse, error) {
	resp := &proto.SubResponse{}
	data, err := statemachine.EncodeUpdate(option)
	if err != nil {
		return nil, err
	}
	result, err := c.cluster.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	// cluster.Write 返回 UpdateResponse envelope，具体格式由 statemachine.StateMachine.Update 定义。
	if err := statemachine.DecodeUpdateResult(result, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteSub 通过 Raft 写入删除订阅。
func (c *Cluster) DeleteSub(ctx context.Context, option *proto.UnSubRequest) (*proto.UnSubResponse, error) {
	resp := &proto.UnSubResponse{}
	data, err := statemachine.EncodeUpdate(option)
	if err != nil {
		return nil, err
	}
	result, err := c.cluster.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	if err := statemachine.DecodeUpdateResult(result, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAllMatchTopics 通过 Raft 读取匹配发布 topic 的订阅过滤器。
func (c *Cluster) GetAllMatchTopics(ctx context.Context, req *proto.GetAllMatchTopicsRequest) (*proto.GetAllMatchTopicsResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := result.(*proto.GetAllMatchTopicsResponse); ok {
		return resp, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", result)
}

// GetAllMatchTopicsForWildTopic 通过 Raft 读取通配过滤器覆盖的订阅 topic。
func (c *Cluster) GetAllMatchTopicsForWildTopic(ctx context.Context, req *proto.GetAllMatchTopicsForWildTopicRequest) (*proto.GetAllMatchTopicsForWildTopicResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := result.(*proto.GetAllMatchTopicsForWildTopicResponse); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("unexpected response type %T", result)
}

// DeleteClient 通过 Raft 写入删除客户端的所有订阅。
func (c *Cluster) DeleteClient(ctx context.Context, req *proto.DeleteClientRequest) (*proto.DeleteClientResponse, error) {
	resp := &proto.DeleteClientResponse{}
	data, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	result, err := c.cluster.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	if err := statemachine.DecodeUpdateResult(result, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteTopic 通过 Raft 写入删除指定 topic filter 下的订阅。
func (c *Cluster) DeleteTopic(ctx context.Context, req *proto.DeleteTopicRequest) (*proto.DeleteTopicResponse, error) {
	resp := &proto.DeleteTopicResponse{}
	data, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	result, err := c.cluster.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	if err := statemachine.DecodeUpdateResult(result, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAllMatchClient 通过 Raft 读取匹配发布 topic 的客户端。
func (c *Cluster) GetAllMatchClient(ctx context.Context, req *proto.GetAllSubTopicClientRequest) (*proto.GetAllSubTopicClientResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := result.(*proto.GetAllSubTopicClientResponse); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("unexpected response type %T", result)

}

// GetAllMatchClientV2 通过 Raft 读取客户端维度的完整匹配结果。
func (c *Cluster) GetAllMatchClientV2(ctx context.Context, req *proto.GetAllMatchClientV2Request) (*proto.GetAllMatchClientV2Response, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp, ok := result.(*proto.GetAllMatchClientV2Response); ok {
		return resp, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", result)
}

// GetSubTree 通过 Raft 读取订阅树快照。
func (c *Cluster) GetSubTree(ctx context.Context, req *proto.GetSubTreeRequest) (*proto.GetSubTreeResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := result.(*proto.GetSubTreeResponse); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("unexpected response type %T", result)
}

// SetClientOwnerToken 通过 Raft 写入设置客户端当前 owner token。
func (c *Cluster) SetClientOwnerToken(ctx context.Context, req *proto.SetClientOwnerTokenRequest) (*proto.SetClientOwnerTokenResponse, error) {
	resp := &proto.SetClientOwnerTokenResponse{}
	data, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	result, err := c.cluster.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	if err := statemachine.DecodeUpdateResult(result, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetClientSubscriptions 通过 Raft 读取客户端当前订阅。
func (c *Cluster) GetClientSubscriptions(ctx context.Context, req *proto.GetClientSubscriptionsRequest) (*proto.GetClientSubscriptionsResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp, ok := result.(*proto.GetClientSubscriptionsResponse); ok {
		return resp, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", result)
}

// GetShareGroupMembers 通过 Raft 读取共享订阅组成员。
func (c *Cluster) GetShareGroupMembers(ctx context.Context, req *proto.GetShareGroupMembersRequest) (*proto.GetShareGroupMembersResponse, error) {
	result, err := c.cluster.Read(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp, ok := result.(*proto.GetShareGroupMembersResponse); ok {
		return resp, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", result)
}
