package session

import (
	"context"

	"github.com/BAN1ce/skyTree/proto/proto_session"
)

// ============================================================================
// 会话中心接口 - 管理客户端元数据和会话
// ============================================================================

// Center 会话中心接口 - 管理客户端元数据和会话数据
type Center interface {
	// OpenSessionForConnect 为新的 CONNECT 打开会话，并返回旧会话和当前生效会话。
	OpenSessionForConnect(ctx context.Context, request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error)
	// TakeOverSessionOwner 接管指定客户端的会话所有者，并返回接管前后的所有者信息。
	TakeOverSessionOwner(ctx context.Context, request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error)
	// ReplaceSessionStateOnCleanStart 按 MQTT Clean Start 语义替换指定客户端的持久会话状态。
	ReplaceSessionStateOnCleanStart(ctx context.Context, request *proto_session.ReplaceSessionStateOnCleanStartRequest) error
	// SaveOfflineState 在连接断开后保存重连所需的离线会话状态。
	SaveOfflineState(ctx context.Context, request *proto_session.SaveOfflineStateRequest) error
	// RemoveOutgoingUnfinished 删除指定客户端已完成的出站未完成消息记录。
	RemoveOutgoingUnfinished(ctx context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error
	// UpsertIncomingUnfinished 保存客户端上行 QoS2 等待 PUBREL 的单条未完成消息。
	UpsertIncomingUnfinished(ctx context.Context, request *proto_session.UpsertIncomingUnfinishedRequest) error
	// RemoveIncomingUnfinished 删除指定客户端已完成的上行 QoS2 未完成消息记录。
	RemoveIncomingUnfinished(ctx context.Context, request *proto_session.RemoveIncomingUnfinishedRequest) error
	// CommitOutgoingProgress 在客户端在线期间周期性提交下行投递进度（QoS1 重放游标 +
	// QoS2/retained 在飞未完成消息），用于把意外宕机后的重放窗口限制在一个提交周期内。
	// 它只更新出站进度，不改动遗嘱、会话过期、上行未完成消息或所有者在线状态。
	CommitOutgoingProgress(ctx context.Context, request *proto_session.CommitOutgoingProgressRequest) error
	// DeleteSession 删除指定客户端的持久会话状态和运行时所有者记录。
	DeleteSession(ctx context.Context, request *proto_session.DeleteSessionRequest) error
	// GetSession 读取指定客户端的会话状态。
	GetSession(ctx context.Context, request *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error)
	// GetSessionOwner 读取指定客户端当前的会话所有者。
	GetSessionOwner(ctx context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error)
	// GetSessionOwners 批量读取多个客户端当前的会话所有者。
	GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error)
}
