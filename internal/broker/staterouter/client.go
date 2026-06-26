package staterouter

import (
	"context"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

// Client is the broker-facing boundary for state and route operations.
type Client interface {
	// AcquireSession 打开或恢复客户端会话，并让当前 broker 获取该会话的 owner 权限。
	// 这里的 Acquire 表示取得会话所有权，可能包含 clean start 状态重置和旧 owner 接管。
	AcquireSession(ctx context.Context, req AcquireSessionRequest) (*AcquireSessionResponse, error)
	// SaveOfflineState 保存客户端断开连接时的离线状态，例如未完成消息、重放游标和遗嘱清理状态。
	SaveOfflineState(ctx context.Context, req SaveOfflineStateRequest) error
	// Subscribe 为当前 owner 持有的客户端保存订阅关系，并返回最终授予的 QoS。
	Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResponse, error)
	// DeleteClientSubscriptions 删除当前 owner 持有客户端的全部订阅关系。
	DeleteClientSubscriptions(ctx context.Context, req DeleteClientSubscriptionsRequest) error
	// ListClientSubscriptions 查询客户端当前保存的订阅关系。
	ListClientSubscriptions(ctx context.Context, req ListClientSubscriptionsRequest) (*ListClientSubscriptionsResponse, error)
	// RoutePublish 根据订阅关系路由客户端发布的消息。
	RoutePublish(ctx context.Context, req RoutePublishRequest) error
	// HasMatchingSubscribers 判断指定主题是否存在可匹配的订阅者。
	HasMatchingSubscribers(ctx context.Context, req HasMatchingSubscribersRequest) (bool, error)
	// ClosePreviousOwner 通知并关闭被当前 broker 接管的旧 owner 连接。
	ClosePreviousOwner(ctx context.Context, req ClosePreviousOwnerRequest) error
}

type AcquireSessionRequest struct {
	BrokerNodeID          uint64
	BrokerInstanceID      string
	ClientID              string
	OwnerToken            string
	WillMessage           *proto_session.WillMessage
	SessionExpiryInterval uint32
	CleanStart            bool
	NowUnixNano           int64
}

type AcquireSessionResponse struct {
	OpenSession *proto_session.OpenSessionForConnectResponse
	Takeover    *proto_session.TakeOverSessionOwnerResponse
}

type SaveOfflineStateRequest struct {
	BrokerNodeID          uint64
	BrokerInstanceID      string
	ClientID              string
	OwnerToken            string
	UnfinishedMessages    []*proto_session.UnfinishedMessage
	OutgoingReplayCursor  *proto_session.OutgoingReplayCursor
	ClearWill             bool
	SessionExpiryInterval uint32
	NowUnixNano           int64
}

type SubscribeRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
	OwnerToken       string
	Subscribe        *packets.Subscribe
	MaxQoS           int
}

type SubscribeResponse struct {
	GrantedQoS []int32
}

type DeleteClientSubscriptionsRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
	OwnerToken       string
}

type ListClientSubscriptionsRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
}

type ListClientSubscriptionsResponse struct {
	Topics map[string]*proto_topic.SubOption
}

type RoutePublishRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
	OwnerToken       string
	Message          *brokerpublish.Message
}

type HasMatchingSubscribersRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
	Topic            string
}

type ClosePreviousOwnerRequest struct {
	BrokerNodeID     uint64
	BrokerInstanceID string
	ClientID         string
	PreviousOwner    *proto_session.SessionOwner
}
