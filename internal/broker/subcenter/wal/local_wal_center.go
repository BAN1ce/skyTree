package wal

import (
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/subcenter/statemachine"
	"github.com/BAN1ce/skyTree/internal/localstate/walsm"
	subscription "github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// LocalWALCenter 是 subscription.Center 的单机持久化实现。
// 所有写请求都会先写入 WAL，并按配置周期性保存完整快照。
type LocalWALCenter struct {
	engine *walsm.Engine
}

var _ subscription.Center = (*LocalWALCenter)(nil)

// NewLocalWALCenter 创建单机 WAL 订阅中心。
func NewLocalWALCenter(baseDir string, snapshotInterval time.Duration, snapshotEntries uint64) (*LocalWALCenter, error) {
	sm := statemachine.NewStateMachine()
	engine, err := walsm.NewEngine(sm, walsm.Options{
		Name:             "sub_center",
		BaseDir:          baseDir,
		SnapshotEntries:  snapshotEntries,
		SnapshotInterval: snapshotInterval,
	})
	if err != nil {
		return nil, err
	}
	return &LocalWALCenter{engine: engine}, nil
}

// Close 关闭底层 WAL 引擎。
func (c *LocalWALCenter) Close() error {
	if c.engine == nil {
		return nil
	}
	return c.engine.Close()
}

// CreateSub 通过 WAL 状态机创建或更新订阅。
func (c *LocalWALCenter) CreateSub(ctx context.Context, option *proto.SubRequest) (*proto.SubResponse, error) {
	updateBytes, err := statemachine.EncodeUpdate(option)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	out := &proto.SubResponse{}
	if err := statemachine.DecodeUpdateResult(res, out); err != nil {
		return nil, err
	}
	_ = ctx
	return out, nil
}

// DeleteSub 通过 WAL 状态机删除订阅。
func (c *LocalWALCenter) DeleteSub(ctx context.Context, option *proto.UnSubRequest) (*proto.UnSubResponse, error) {
	updateBytes, err := statemachine.EncodeUpdate(option)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	out := &proto.UnSubResponse{}
	if err := statemachine.DecodeUpdateResult(res, out); err != nil {
		return nil, err
	}
	_ = ctx
	return out, nil
}

// DeleteClient 通过 WAL 状态机删除客户端的所有订阅。
func (c *LocalWALCenter) DeleteClient(ctx context.Context, req *proto.DeleteClientRequest) (*proto.DeleteClientResponse, error) {
	updateBytes, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	out := &proto.DeleteClientResponse{}
	if err := statemachine.DecodeUpdateResult(res, out); err != nil {
		return nil, err
	}
	_ = ctx
	return out, nil
}

// DeleteTopic 通过 WAL 状态机删除指定 topic filter 下的订阅。
func (c *LocalWALCenter) DeleteTopic(ctx context.Context, req *proto.DeleteTopicRequest) (*proto.DeleteTopicResponse, error) {
	updateBytes, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	out := &proto.DeleteTopicResponse{}
	if err := statemachine.DecodeUpdateResult(res, out); err != nil {
		return nil, err
	}
	_ = ctx
	return out, nil
}

// SetClientOwnerToken 通过 WAL 状态机设置客户端 owner token。
func (c *LocalWALCenter) SetClientOwnerToken(ctx context.Context, req *proto.SetClientOwnerTokenRequest) (*proto.SetClientOwnerTokenResponse, error) {
	updateBytes, err := statemachine.EncodeUpdate(req)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	out := &proto.SetClientOwnerTokenResponse{}
	if err := statemachine.DecodeUpdateResult(res, out); err != nil {
		return nil, err
	}
	_ = ctx
	return out, nil
}

// GetAllMatchTopics 从 WAL 状态机读取匹配发布 topic 的订阅过滤器。
func (c *LocalWALCenter) GetAllMatchTopics(ctx context.Context, req *proto.GetAllMatchTopicsRequest) (*proto.GetAllMatchTopicsResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetAllMatchTopicsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetAllMatchTopicsForWildTopic 从 WAL 状态机读取通配过滤器覆盖的订阅 topic。
func (c *LocalWALCenter) GetAllMatchTopicsForWildTopic(ctx context.Context, req *proto.GetAllMatchTopicsForWildTopicRequest) (*proto.GetAllMatchTopicsForWildTopicResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetAllMatchTopicsForWildTopicResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetAllMatchClient 从 WAL 状态机读取匹配发布 topic 的客户端。
func (c *LocalWALCenter) GetAllMatchClient(ctx context.Context, req *proto.GetAllSubTopicClientRequest) (*proto.GetAllSubTopicClientResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetAllSubTopicClientResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetAllMatchClientV2 从 WAL 状态机读取客户端维度的完整匹配结果。
func (c *LocalWALCenter) GetAllMatchClientV2(ctx context.Context, req *proto.GetAllMatchClientV2Request) (*proto.GetAllMatchClientV2Response, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetAllMatchClientV2Response)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetSubTree 从 WAL 状态机读取订阅树快照。
func (c *LocalWALCenter) GetSubTree(ctx context.Context, req *proto.GetSubTreeRequest) (*proto.GetSubTreeResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetSubTreeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetClientSubscriptions 从 WAL 状态机读取客户端当前订阅。
func (c *LocalWALCenter) GetClientSubscriptions(ctx context.Context, req *proto.GetClientSubscriptionsRequest) (*proto.GetClientSubscriptionsResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetClientSubscriptionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

// GetShareGroupMembers 从 WAL 状态机读取共享订阅组成员。
func (c *LocalWALCenter) GetShareGroupMembers(ctx context.Context, req *proto.GetShareGroupMembersRequest) (*proto.GetShareGroupMembersResponse, error) {
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto.GetShareGroupMembersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}
