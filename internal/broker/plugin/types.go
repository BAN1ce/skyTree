package plugin

import (
	"context"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// 插件函数类型定义

// 连接相关插件
type OnReceivedConnect func(ctx context.Context, clientID string, connect *packets.Connect) error
type OnSendConnAck func(ctx context.Context, clientID string, connAck *packets.ConnAck) error
type OnReceivedDisconnect func(ctx context.Context, clientID string, disconnect *packets.Disconnect) error

// 订阅相关插件
type OnSubscribe func(ctx context.Context, clientID string, subscribe *packets.Subscribe) error
type OnSendSubAck func(ctx context.Context, clientID string, subAck *packets.Suback) error
type OnUnsubscribe func(ctx context.Context, clientID string, unsubscribe *packets.Unsubscribe) error
type OnSendUnsubAck func(ctx context.Context, clientID string, unsubAck *packets.Unsuback) error

// 发布相关插件
type OnReceivedPublish func(ctx context.Context, clientID string, publish *packets.Publish) error
type OnSendPublish func(ctx context.Context, clientID string, publish *packets.Publish) error

// QoS确认相关插件
type OnReceivedPubAck func(ctx context.Context, clientID string, pubAck *packets.Puback) error
type OnSendPubAck func(ctx context.Context, clientID string, pubAck *packets.Puback) error
type OnReceivedPubRel func(ctx context.Context, clientID string, pubRel *packets.Pubrel) error
type OnSendPubRel func(ctx context.Context, clientID string, pubRel *packets.Pubrel) error
type OnReceivedPubRec func(ctx context.Context, clientID string, pubRec *packets.Pubrec) error
type OnSendPubRec func(ctx context.Context, clientID string, pubRec *packets.Pubrec) error
type OnReceivedPubComp func(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error
type OnSendPubComp func(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error

// 心跳相关插件
type OnReceivedPingReq func(ctx context.Context, clientID string, pingReq *packets.Pingreq) error
type OnSendPingResp func(ctx context.Context, clientID string, pingResp *packets.Pingresp) error

// AUTH相关插件
type OnReceivedAuth func(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error)
type OnSendAuth func(ctx context.Context, clientID string, auth *packets.Auth) error

// 客户端状态插件
type OnClientOnline func(clientID string)
type OnClientOffline func(clientID string)

// 错误处理插件
type OnClientError func(ctx context.Context, clientID string, err error) error

// 插件结构体定义
type PacketPlugin struct {
	// 连接相关
	OnReceivedConnect    []OnReceivedConnect
	OnSendConnAck        []OnSendConnAck
	OnReceivedDisconnect []OnReceivedDisconnect

	// 订阅相关
	OnSubscribe    []OnSubscribe
	OnSendSubAck   []OnSendSubAck
	OnUnsubscribe  []OnUnsubscribe
	OnSendUnsubAck []OnSendUnsubAck

	// 发布相关
	OnReceivedPublish []OnReceivedPublish
	OnSendPublish     []OnSendPublish

	// QoS确认相关
	OnReceivedPubAck  []OnReceivedPubAck
	OnSendPubAck      []OnSendPubAck
	OnReceivedPubRel  []OnReceivedPubRel
	OnSendPubRel      []OnSendPubRel
	OnReceivedPubRec  []OnReceivedPubRec
	OnSendPubRec      []OnSendPubRec
	OnReceivedPubComp []OnReceivedPubComp
	OnSendPubComp     []OnSendPubComp

	// 心跳相关
	OnReceivedPingReq []OnReceivedPingReq
	OnSendPingResp    []OnSendPingResp

	// AUTH相关
	OnReceivedAuth []OnReceivedAuth
	OnSendAuth     []OnSendAuth
}

type ClientPlugin struct {
	OnClientOnline  []OnClientOnline
	OnClientOffline []OnClientOffline
	OnClientError   []OnClientError
}

type Plugins struct {
	PacketPlugin
	ClientPlugin
}
