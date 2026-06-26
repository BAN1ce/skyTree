package wrapper

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// SubCenterWrapper 包装 subscription.Center，并在关键读写路径上补充指标观测。
type SubCenterWrapper struct {
	subscribeCenter subscription.Center
}

// NewSubCenterWrapper 创建订阅中心包装器。
func NewSubCenterWrapper(center subscription.Center) *SubCenterWrapper {
	return &SubCenterWrapper{
		subscribeCenter: center,
	}
}

// CreateSub 创建或更新订阅，并记录写入耗时指标。
func (w *SubCenterWrapper) CreateSub(ctx context.Context, option *proto.SubRequest) (*proto.SubResponse, error) {
	var (
		start     = time.Now()
		resp, err = w.subscribeCenter.CreateSub(ctx, option)
	)
	metric.RecordSubCenterRequest("write", err, time.Since(start))

	return resp, err

}

// DeleteSub 删除订阅，并记录删除耗时指标。
func (w *SubCenterWrapper) DeleteSub(ctx context.Context, option *proto.UnSubRequest) (*proto.UnSubResponse, error) {
	var (
		start     = time.Now()
		resp, err = w.subscribeCenter.DeleteSub(ctx, option)
	)
	metric.RecordSubCenterRequest("delete", err, time.Since(start))

	return resp, err
}

// GetAllMatchTopics 查询匹配发布 topic 的订阅过滤器，并记录读取耗时指标。
func (w *SubCenterWrapper) GetAllMatchTopics(ctx context.Context, req *proto.GetAllMatchTopicsRequest) (*proto.GetAllMatchTopicsResponse, error) {
	var (
		start     = time.Now()
		resp, err = w.subscribeCenter.GetAllMatchTopics(ctx, req)
	)
	metric.RecordSubCenterRequest("read", err, time.Since(start))
	return resp, err
}

// GetAllMatchTopicsForWildTopic 转发通配过滤器覆盖查询。
func (w *SubCenterWrapper) GetAllMatchTopicsForWildTopic(ctx context.Context, req *proto.GetAllMatchTopicsForWildTopicRequest) (*proto.GetAllMatchTopicsForWildTopicResponse, error) {
	return w.subscribeCenter.GetAllMatchTopicsForWildTopic(ctx, req)
}

// DeleteClient 转发客户端订阅清理请求。
func (w *SubCenterWrapper) DeleteClient(ctx context.Context, req *proto.DeleteClientRequest) (*proto.DeleteClientResponse, error) {
	return w.subscribeCenter.DeleteClient(ctx, req)
}

// DeleteTopic 转发 topic filter 订阅清理请求。
func (w *SubCenterWrapper) DeleteTopic(ctx context.Context, req *proto.DeleteTopicRequest) (*proto.DeleteTopicResponse, error) {
	return w.subscribeCenter.DeleteTopic(ctx, req)
}

// GetAllMatchClient 转发客户端匹配查询。
func (w *SubCenterWrapper) GetAllMatchClient(ctx context.Context, req *proto.GetAllSubTopicClientRequest) (*proto.GetAllSubTopicClientResponse, error) {
	return w.subscribeCenter.GetAllMatchClient(ctx, req)
}

// GetAllMatchClientV2 转发客户端维度的完整匹配查询。
func (w *SubCenterWrapper) GetAllMatchClientV2(ctx context.Context, req *proto.GetAllMatchClientV2Request) (*proto.GetAllMatchClientV2Response, error) {
	return w.subscribeCenter.GetAllMatchClientV2(ctx, req)
}

// GetSubTree 转发订阅树快照查询。
func (w *SubCenterWrapper) GetSubTree(ctx context.Context, req *proto.GetSubTreeRequest) (*proto.GetSubTreeResponse, error) {
	return w.subscribeCenter.GetSubTree(ctx, req)
}

// SetClientOwnerToken 转发客户端 owner token 设置请求。
func (w *SubCenterWrapper) SetClientOwnerToken(ctx context.Context, req *proto.SetClientOwnerTokenRequest) (*proto.SetClientOwnerTokenResponse, error) {
	return w.subscribeCenter.SetClientOwnerToken(ctx, req)
}

// GetClientSubscriptions 转发客户端订阅列表查询。
func (w *SubCenterWrapper) GetClientSubscriptions(ctx context.Context, req *proto.GetClientSubscriptionsRequest) (*proto.GetClientSubscriptionsResponse, error) {
	return w.subscribeCenter.GetClientSubscriptions(ctx, req)
}

// GetShareGroupMembers 转发共享订阅组成员查询。
func (w *SubCenterWrapper) GetShareGroupMembers(ctx context.Context, req *proto.GetShareGroupMembersRequest) (*proto.GetShareGroupMembersResponse, error) {
	return w.subscribeCenter.GetShareGroupMembers(ctx, req)
}
