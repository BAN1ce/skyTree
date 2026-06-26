package core

import (
	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/facade"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/kataras/go-events"
)

type Options struct {
}

type Option func(*Broker)

func WithGlobalNotify(notify cluster.NodeController) Option {
	return func(b *Broker) {
		b.integrations.nodeController = notify
	}
}
func WithNodeMeta(meta *cluster.NodeMeta) Option {
	return func(b *Broker) {
		b.cluster.nodeMeta = meta
	}
}

func WithClientCenter(center session.Center) Option {
	return func(b *Broker) {
		b.state.sessionCenter = center
	}
}

func WithClusterController(controller cluster.State) Option {
	return func(b *Broker) {
		b.cluster.nodeState = controller
	}
}

func WithClientDeliveryEvent(ev delivery_notify.ClientDeliveryEvent) Option {
	return func(b *Broker) {
		b.delivery.event = ev
	}
}

func WithEvent(driver events.EventEmmiter) Option {
	return func(b *Broker) {
		b.integrations.event = driver
	}
}

func WithClientManager(manager *client.Manager) Option {
	return func(b *Broker) {
		b.clients.manager = manager
	}
}

func WithSubCenter(tree subscription.Center) Option {
	return func(b *Broker) {
		b.state.subCenter = tree
	}
}

func WithHandlers(handlers *Handlers) Option {
	return func(b *Broker) {
		b.pluginSet.handlers = handlers
	}
}

func WithPublishRetry(schedule facade.RetrySchedule) Option {
	return func(b *Broker) {
		b.publish.retry = schedule
	}
}

func WithPlugins(plugins *plugin.Plugins) Option {
	return func(b *Broker) {
		b.pluginSet.hooks = plugins
	}
}

func WithKeyStore(store store.KVStore) Option {
	return func(b *Broker) {
		b.state.keyStore = store
	}
}

func WithRetainStore(retain *retain.Store) Option {
	return func(b *Broker) {
		b.state.retain = retain
	}
}

func WithStateRouter(router staterouter.Client) Option {
	return func(b *Broker) {
		b.state.router = router
	}
}

func WithDeliveryTaskStore(taskStore delivery.TaskStore) Option {
	return func(b *Broker) {
		b.delivery.taskStore = taskStore
	}
}

func WithDeliveryCursorStore(cursorStore delivery.CursorStore) Option {
	return func(b *Broker) {
		b.delivery.cursorStore = cursorStore
	}
}

// Scanner will be created in Start() when ctx is available
func WithWillDelayCenter(center willdelay.Center, sessionCenter session.Center, cluster *raft.Cluster, clusterID uint64, localNodeID uint64) Option {
	return func(b *Broker) {
		b.will.delayCenter = center
		b.cluster.raft = cluster
		b.will.delaySessionCenter = sessionCenter
		b.will.delayClusterID = clusterID
		b.will.delayLocalNodeID = localNodeID
	}
}

func WithSharedSubscriptionStore(ss store.SharedSubscriptionStore) Option {
	return func(b *Broker) {
		b.shared.store = ss
	}
}
