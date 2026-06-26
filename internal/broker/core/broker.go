package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/acl"
	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/clientalive"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	"github.com/BAN1ce/skyTree/internal/broker/server"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	will_delay2 "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/facade"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetpool"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/kataras/go-events"
)

type Observer interface {
	OnClientClose(b Broker, c *brokerclient.Client)
}

type Handlers struct {
	Connect     brokerHandler
	Publish     brokerHandler
	PublishAck  brokerHandler
	PublishRec  brokerHandler
	PublishRel  brokerHandler
	PublishComp brokerHandler
	Ping        brokerHandler
	Sub         brokerHandler
	UnSub       brokerHandler
	Auth        brokerHandler
	Disconnect  brokerHandler
}

type brokerHandler interface {
	Handle(broker *Broker, client *brokerclient.Client, rawPacket *packets.ControlPacket) (err error)
}

type Broker struct {
	runtime      brokerRuntimeState
	network      brokerNetworkResources
	state        brokerStateCenters
	delivery     brokerDeliveryResources
	clients      brokerClientResources
	publish      brokerPublishResources
	pluginSet    brokerPluginResources
	cluster      brokerClusterResources
	config       brokerConfigSet
	integrations brokerIntegrationResources
	will         brokerWillResources
	shared       brokerSharedSubscriptionResources
}

// brokerRuntimeState 保存 Broker 生命周期控制相关状态。
type brokerRuntimeState struct {
	ctx    context.Context
	cancel context.CancelFunc

	shuttingDown atomic.Bool
}

// brokerNetworkResources 保存 listener 和当前接入连接，统一由 network.mux 保护。
type brokerNetworkResources struct {
	mux         sync.RWMutex
	server      *server.Server
	connections map[net.Conn]struct{}
}

// brokerStateCenters 保存 Broker 依赖的状态中心和持久化入口。
type brokerStateCenters struct {
	subCenter     subscription.Center
	sessionCenter session.Center
	keyStore      store.KVStore
	retain        *retain.Store
	router        staterouter.Client
}

// brokerDeliveryResources 保存下行投递流水线的队列、游标和跨节点通知依赖。
type brokerDeliveryResources struct {
	// 投递流水线存储由 app 层按当前存储后端注入，Broker 只依赖接口。
	taskStore   delivery.TaskStore
	cursorStore delivery.CursorStore
	event       delivery_notify.ClientDeliveryEvent
}

// brokerClientResources 保存在线 client 管理和 client 后台任务生命周期。
type brokerClientResources struct {
	manager          *brokerclient.Manager
	keepAliveTracker *clientalive.Tracker

	wg               sync.WaitGroup
	backgroundTaskWg sync.WaitGroup
}

// brokerPublishResources 保存发布路径复用对象和重试调度入口。
type brokerPublishResources struct {
	pool        *packetpool.Publish
	retry       facade.RetrySchedule
	retryWorker *facade.PublishRetry
}

// brokerPluginResources 保存协议处理器、插件链和插件派生出的管理器。
type brokerPluginResources struct {
	handlers   *Handlers
	hooks      *plugin.Plugins
	aclManager *acl.Manager
}

// brokerClusterResources 保存集群状态和 Raft 依赖。
type brokerClusterResources struct {
	nodeState cluster.State
	nodeMeta  *cluster.NodeMeta
	raft      *raft.Cluster
}

// brokerConfigSet 保存 Broker 启动后需要传递给子组件的配置快照。
type brokerConfigSet struct {
	broker   config.Broker
	plugins  config.Plugins
	cluster  config.Cluster
	delivery config.DeliveryRunner
}

// brokerIntegrationResources 保存跨模块集成依赖。
type brokerIntegrationResources struct {
	event          events.EventEmmiter
	nodeController cluster.NodeController
}

// brokerWillResources 保存遗嘱消息队列和 will delay 扫描依赖。
type brokerWillResources struct {
	messageChan chan *brokerpublish.Message

	delayCenter        will_delay2.Center
	delayScanner       *will_delay2.Scanner
	delaySessionCenter session.Center
	delayClusterID     uint64
	delayLocalNodeID   uint64
}

// brokerSharedSubscriptionResources 保存共享订阅队列和消费协调器。
type brokerSharedSubscriptionResources struct {
	store    store.SharedSubscriptionStore
	manager  *shared_manager.SharedSubscriptionManager
	notifier sharedTaskNotifier
}

type sharedTaskNotifier interface {
	NotifyTaskAppended(ctx context.Context, event shared_manager.TaskAppendedEvent) error
}

func (r brokerSharedSubscriptionResources) taskNotifier() sharedTaskNotifier {
	if r.notifier != nil {
		return r.notifier
	}
	return r.manager
}

// sharedSubClientManagerAdapter 适配共享订阅 consumer 对在线 client 查询的最小接口。
// consumer 只需要判断 client 是否在线，因此返回真实 client 指针作为 interface{}。
type sharedSubClientManagerAdapter struct {
	m *brokerclient.Manager
}

func (a sharedSubClientManagerAdapter) ReadClient(id string) (interface{}, bool) {
	if a.m == nil {
		return nil, false
	}
	return a.m.ReadClient(id)
}

// NewBroker 创建 Broker 核心实例，并完成 listener、基础缓存和插件依赖的初始化。
func NewBroker(
	cfg config.Broker,
	pluginCfg config.Plugins,
	clusterCfg config.Cluster,
	deliveryCfg config.DeliveryRunner,
	option ...Option,
) (*Broker, error) {
	var (
		b = &Broker{
			config: brokerConfigSet{
				broker:   cfg,
				plugins:  pluginCfg,
				cluster:  clusterCfg,
				delivery: deliveryCfg,
			},
			publish: brokerPublishResources{
				pool: packetpool.NewPublish(),
			},
			network: brokerNetworkResources{
				connections: make(map[net.Conn]struct{}, 50000),
			},
			will: brokerWillResources{
				messageChan: make(chan *brokerpublish.Message, 1000),
			},
		}
	)

	var err error
	b.network.server, err = server.NewServer(
		b.config.broker.Listen,
		server.WithTLSFilesAndMTLS(
			b.config.broker.TLS.CertFile,
			b.config.broker.TLS.KeyFile,
			b.config.broker.TLS.CAFile,
			b.config.broker.TLS.ReloadInterval,
			b.config.broker.TLS.MTLSAuthMode,
		),
	)
	if err != nil {
		return nil, err
	}

	for _, opt := range option {
		opt(b)
	}

	b.clients.keepAliveTracker = clientalive.NewTracker()
	if b.publish.retry == nil {
		b.publish.retryWorker = facade.NewPublishRetry(
			b,
			retry.WithInterval(b.config.broker.MessageRetry.SchedulerInterval),
		)
		if b.publish.retryWorker == nil {
			return nil, fmt.Errorf("create publish retry worker failed")
		}
		b.publish.retry = b.publish.retryWorker
	}

	b.attachACLPlugin()

	return b, nil
}

// attachACLPlugin 在 ACL 开关打开时挂载连接、订阅和发布阶段的权限校验插件。
func (b *Broker) attachACLPlugin() {
	if !b.config.plugins.ACL.Enabled {
		return
	}
	if b.pluginSet.hooks == nil {
		b.pluginSet.hooks = &plugin.Plugins{}
	}
	if b.state.keyStore == nil {
		logger.Logger.Error().Msg("ACL enabled but keyStore is nil; ACL will fail closed")
	}

	aclCfg := b.config.plugins.ACL
	mgr := acl.NewManager(acl.ManagerConfig{
		Store:       b.state.keyStore,
		KeyStoreKey: aclCfg.KeyStoreKey,
		FilePath:    aclCfg.File,
		DefaultDeny: aclCfg.DefaultDeny,
	})
	b.pluginSet.aclManager = mgr
	aclPlugin := plugin.NewACLPluginWithManager(mgr)
	if aclPlugin == nil {
		logger.Logger.Error().Msg("failed to create ACL plugin")
		return
	}
	b.pluginSet.hooks.OnReceivedConnect = append(b.pluginSet.hooks.OnReceivedConnect, aclPlugin.OnReceivedConnect)
	b.pluginSet.hooks.OnSubscribe = append(b.pluginSet.hooks.OnSubscribe, aclPlugin.OnSubscribe)
	b.pluginSet.hooks.OnReceivedPublish = append(b.pluginSet.hooks.OnReceivedPublish, aclPlugin.OnReceivedPublish)

	logger.Logger.Info().Str("file", aclCfg.File).Str("keystore_key", aclCfg.KeyStoreKey).
		Msg(fmt.Sprintf("ACL plugin enabled (default_deny=%t)", aclCfg.DefaultDeny))
}

func (b *Broker) ACLManager() *acl.Manager {
	if b == nil {
		return nil
	}
	return b.pluginSet.aclManager
}

func (b *Broker) SharedSubscriptionManager() *shared_manager.SharedSubscriptionManager {
	if b == nil {
		return nil
	}
	return b.shared.manager
}

func (b *Broker) SetPublishRetry(schedule facade.RetrySchedule) {
	if b == nil {
		return
	}
	b.publish.retry = schedule
	if worker, ok := schedule.(*facade.PublishRetry); ok {
		b.publish.retryWorker = worker
		return
	}
	b.publish.retryWorker = nil
}
