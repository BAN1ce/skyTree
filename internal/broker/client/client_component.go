package client

import (
	"context"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/clientalive"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/facade"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

type ComponentOption func(*Component)

type ConnectAdmission func(context.Context, string, *packets.Connect) ConnectAdmissionResult

type ConnectAdmissionResult struct {
	ReasonCode      byte
	ReasonString    string
	ServerReference string
}

type Component struct {
	cfg Config

	lifecycleCtx context.Context

	backgroundTaskWG *sync.WaitGroup

	// Delivery cursor store is injected from App based on the selected DB backend.
	deliveryCursorStore delivery.CursorStore

	clientDeliveryEvent delivery_notify.ClientDeliveryEvent

	sessionCenter session.Center

	retain *retain.Store

	plugin *plugin.Plugins

	subCenter subscription.Center

	clientManager *Manager

	keepAliveTracker *clientalive.Tracker

	notifyWillMessageChan chan<- *brokerpublish.Message

	closeClient cluster.NodeController

	stateRouter staterouter.Client

	willDelayCenter willdelay.Center

	sharedSubscriptionManager *shared_manager.SharedSubscriptionManager

	connectAdmission ConnectAdmission

	publishRetry facade.RetrySchedule
}

func WithClientManager(manager *Manager) ComponentOption {
	return func(c *Component) {
		c.clientManager = manager
	}
}

func WithClosClient(closeClient cluster.NodeController) ComponentOption {
	return func(component *Component) {
		component.closeClient = closeClient
	}

}

func WithNotifyWillMessageChan(ch chan<- *brokerpublish.Message) ComponentOption {
	return func(component *Component) {
		component.notifyWillMessageChan = ch
	}
}

func WithStateRouter(router staterouter.Client) ComponentOption {
	return func(component *Component) {
		component.stateRouter = router
	}
}

func WithKeepAliveTracker(tracker *clientalive.Tracker) ComponentOption {
	return func(c *Component) {
		c.keepAliveTracker = tracker
	}
}

func WithSessionCenter(center session.Center) ComponentOption {
	return func(component *Component) {
		component.sessionCenter = center
	}
}

func WithDeliveryCursorStore(cursorStore delivery.CursorStore) ComponentOption {
	return func(options *Component) {
		options.deliveryCursorStore = cursorStore
	}
}

func WithClientDeliveryEvent(ev delivery_notify.ClientDeliveryEvent) ComponentOption {
	return func(options *Component) {
		options.clientDeliveryEvent = ev
	}
}

func WithRetain(retain2 *retain.Store) ComponentOption {
	return func(options *Component) {
		options.retain = retain2
	}
}

func WithConfig(cfg Config) ComponentOption {
	return func(options *Component) {
		cfg.provided = true
		options.cfg = cfg
	}
}

func WithLifecycleContext(ctx context.Context) ComponentOption {
	return func(options *Component) {
		options.lifecycleCtx = ctx
	}
}

func WithBackgroundTaskWaitGroup(wg *sync.WaitGroup) ComponentOption {
	return func(options *Component) {
		options.backgroundTaskWG = wg
	}
}

func WithPlugin(plugins *plugin.Plugins) ComponentOption {
	return func(options *Component) {
		options.plugin = plugins
	}
}

func WithKeepAliveTime(keepAlive time.Duration) ComponentOption {
	return func(options *Component) {
		options.cfg.KeepAlive = keepAlive
	}
}

func WithSubCenter(subCenter subscription.Center) ComponentOption {
	return func(c *Component) {
		c.subCenter = subCenter
	}
}

func WithWillDelayCenter(center willdelay.Center) ComponentOption {
	return func(component *Component) {
		component.willDelayCenter = center
	}
}

func WithSharedSubscriptionManager(manager *shared_manager.SharedSubscriptionManager) ComponentOption {
	return func(component *Component) {
		component.sharedSubscriptionManager = manager
	}
}

func WithConnectAdmission(admission ConnectAdmission) ComponentOption {
	return func(component *Component) {
		component.connectAdmission = admission
	}
}

func WithPublishRetry(schedule facade.RetrySchedule) ComponentOption {
	return func(component *Component) {
		component.publishRetry = schedule
	}
}
