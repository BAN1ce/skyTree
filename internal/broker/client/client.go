package client

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client/qos2receive"
	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
)

type Config struct {
	WindowSize       int
	ReadStoreTimeout time.Duration
	WriteTimeout     time.Duration
	KeepAlive        time.Duration
	BrokerConfig     config.Broker
	// BrokerConfigResolved indicates BrokerConfig has already gone through
	// full config loading/defaulting and should be used as-is.
	BrokerConfigResolved bool
	ClusterConfig        config.Cluster
	DeliveryConfig       config.DeliveryRunner

	provided bool
}

const mqttNeverExpireSessionInterval = ^uint32(0)

// errOversizedOutboundPublish 由 Client.write 在下行 PUBLISH 超过客户端 Maximum Packet Size 时返回。
// 上层（delivery_runner / 重传逻辑）必须把它当作"已成功处理（丢弃）"，而不是连接错误。
var errOversizedOutboundPublish = errors.New("outbound publish exceeds client maximum packet size; discarded")

// IsOversizedOutboundPublish 报告 err 是否表示下行 PUBLISH 因超长被丢弃。
func IsOversizedOutboundPublish(err error) bool {
	return errors.Is(err, errOversizedOutboundPublish)
}

type enhancedAuthState byte

const (
	enhancedAuthNone enhancedAuthState = iota
	enhancedAuthAuthenticating
	enhancedAuthAuthenticated
	enhancedAuthReauthenticating
)

type Handler interface {
	HandlePacket(context.Context, *packets.ControlPacket, *Client) error
}

// Client 表示一个 MQTT 客户端连接的运行时状态。
//
// 它同时持有网络连接、协议协商结果、会话生命周期状态和下行投递状态。
// 多数业务逻辑按职责拆到同 package 的其它文件中，通过 *Client receiver 共享这些状态。
type Client struct {
	// mux 保护连接生命周期相关的关闭流程，避免重复关闭或并发修改关键状态。
	mux sync.RWMutex

	// ctx 是该连接的生命周期上下文；连接关闭时会被 cancel。
	ctx context.Context

	// conn 是当前客户端连接对应的底层网络连接。
	conn net.Conn

	// ID 是 MQTT ClientID。
	ID string `json:"id"`

	// Username 是 CONNECT 阶段携带的用户名。
	Username string `json:"username"`

	// cancel 用于结束 client 生命周期，并保留关闭原因。
	cancel context.CancelCauseFunc

	// handler 保存当前连接处理 MQTT 控制包的处理链。
	handler []Handler

	// component 聚合 broker 注入给 client 的外部依赖。
	component *Component

	// packetIDFactory 负责为下行 QoS1/QoS2 包分配 MQTT Packet Identifier。
	packetIDFactory clientcap.PacketIDGenerator

	// publishBucket 表示客户端声明的下行 Receive Maximum 窗口。
	publishBucket *rate.Bucket

	// incomingPublishRateLimiter 限制 client -> broker 的 PUBLISH 速率。
	incomingPublishRateLimiter *incomingPublishRateLimiter

	// shareTopic 记录当前 client 参与的共享订阅主题。
	shareTopic map[string]struct{}

	// packetIdentifierIDTopic 维护 PacketID 与 topic 的关联，用于释放和校验。
	packetIdentifierIDTopic *PacketIDTopic

	// QoS2 保存上行 QoS2 已进入两阶段握手但尚未完成的消息。
	QoS2 *qos2receive.QoS2ReceiveStore

	// topicAliasManager 管理 MQTT5 Topic Alias，上行和下行共用一个管理器。
	topicAliasManager *TopicAliasManager

	// clientMaximumPacketSize 是客户端在 CONNECT 中声明的最大可接收包大小。
	clientMaximumPacketSize *uint32

	// serverReceiveMaximum 是服务端在 CONNACK 中声明的上行 inflight 窗口。
	serverReceiveMaximum uint16
	// incomingQoS1Inflight 记录尚未成功写出 PUBACK 的上行 QoS1 PacketID。
	// 只有 PUBACK 写入 socket 后才释放配额，避免处理函数短暂返回导致 inflight 计数偏小。
	incomingQoS1Inflight map[uint16]struct{}
	// incomingQoS1MessageIDs 保存上行 QoS1 PacketID 对应的逻辑消息 ID。
	incomingQoS1MessageIDs map[uint16]uuid.UUID

	// requestProblemInfo 表示客户端是否请求 MQTT5 Problem Information。
	requestProblemInfo bool

	// enhancedAuthMethod 是 MQTT5 Enhanced Auth 的认证方法名。
	enhancedAuthMethod string

	// enhancedAuthData 保存 Enhanced Auth 当前阶段的认证数据。
	enhancedAuthData []byte

	// enhancedAuthState 记录 Enhanced Auth 的状态机阶段。
	enhancedAuthState enhancedAuthState

	// pendingEnhancedAuthConnect 保存等待增强认证完成的 CONNECT 包。
	pendingEnhancedAuthConnect *packets.Connect

	// pendingEnhancedAuthAssignedByServer 表示 pending CONNECT 是否由服务端分配 ClientID。
	pendingEnhancedAuthAssignedByServer bool

	// ownerToken 是绑定当前活跃连接的 fencing token，避免旧节点或旧请求修改新连接。
	ownerToken string

	// cleanSession 记录 MQTT3 clean session / MQTT5 clean start 相关会话语义。
	cleanSession bool

	// sessionExpiryInterval 是本次连接协商后的 Session Expiry Interval。
	sessionExpiryInterval uint32

	// connectSessionExpiryInterval 是 CONNECT 包中携带的原始 Session Expiry Interval。
	connectSessionExpiryInterval uint32

	// keepAlive 是当前连接协商后的 Keep Alive 超时时间。
	keepAlive time.Duration

	// aliveTime 保存最近一次收到客户端包的时间，用于 keepalive 判断。
	aliveTime atomic.Value

	// closeOnce 保证 close 流程只执行一次。
	closeOnce sync.Once

	// writeMux 串行化 socket 写入，避免 MQTT 控制包交错。
	writeMux sync.Mutex

	// willMessage 保存当前 session 的遗嘱消息。
	willMessage *brokerpublish.Message

	// disconnectWithoutWill 记录关闭原因，用于判断是否抑制遗嘱发布。
	disconnectWithoutWill atomic.Int64

	// connAckAccepted 标记 CONNACK 是否已经成功写出。
	// MQTT5 禁止服务端在成功 CONNACK 前发送 DISCONNECT。
	connAckAccepted atomic.Bool

	// willScheduledAtSessionExpiry 表示已通过 willDelayCenter 安排 session 过期时发布遗嘱。
	willScheduledAtSessionExpiry atomic.Bool

	// deliveryRunnerOnce 保证每个连接只启动一个 client delivery runner。
	deliveryRunnerOnce sync.Once

	// deliveryWakeCh 用于唤醒当前 client 的投递 runner 读取新任务。
	deliveryWakeCh chan struct{}
	// deliveryListenerID 是注册到本地投递事件中心的监听器 ID。
	deliveryListenerID string
	// deliveryListenerOnce 保证投递事件监听只注册一次。
	deliveryListenerOnce sync.Once

	// outgoingInflight 保存下行 QoS1/QoS2 已发送但尚未完成 ACK 流程的消息。
	outgoingInflight *outgoingInflightStore

	// outgoingReplayCursor 记录普通下行 QoS1 断线后需要从哪个 delivery task 开始重放。
	outgoingReplayCursor *outgoingReplayCursor

	// 下行投递进度的周期性提交状态（详见 commitOutgoingProgressToSession）。
	// outgoingAcksSinceCommit 统计自上次提交以来的下行终态 ACK 数（QoS1 PUBACK / QoS2 PUBCOMP）。
	outgoingAcksSinceCommit atomic.Int64
	// outgoingCommitInFlight 保证同一时刻只有一个进度提交在写，避免并发与写放大。
	outgoingCommitInFlight atomic.Bool
	// lastOutgoingCommitUnixNano 记录上次成功触发提交的时间，用于时间维度触发。
	lastOutgoingCommitUnixNano atomic.Int64
}

// NewClient 创建一个 MQTT client 运行时对象，并初始化协议状态与投递状态。
func NewClient(conn net.Conn, option ...ComponentOption) *Client {
	var (
		c = &Client{
			conn:                    conn,
			component:               new(Component),
			packetIdentifierIDTopic: NewPacketIDTopic(),
			shareTopic:              map[string]struct{}{},
			outgoingInflight:        newOutgoingInflightStore(),
			incomingQoS1Inflight:    make(map[uint16]struct{}),
			incomingQoS1MessageIDs:  make(map[uint16]uuid.UUID),
			requestProblemInfo:      true,
		}
	)
	c.disconnectWithoutWill.Store(-1)
	for _, o := range option {
		o(c.component)
	}
	c.packetIDFactory = NewPacketIDFactory()

	c.QoS2 = qos2receive.NewQoS2ReceiveStore()
	c.topicAliasManager = NewTopicAliasManager()

	return c
}

// GetConn 返回当前 client 的底层网络连接。
func (c *Client) GetConn() net.Conn {
	return c.conn
}

// GetCtx 返回当前 client 的生命周期上下文。
func (c *Client) GetCtx() context.Context {
	return c.getCtx()
}

// getCtx 在不加锁的情况下返回 client 生命周期上下文。
func (c *Client) getCtx() context.Context {
	return c.ctx
}
