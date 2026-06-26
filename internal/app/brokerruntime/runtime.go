package brokerruntime

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/internal/app/statecenter"
	"github.com/BAN1ce/skyTree/internal/app/storeruntime"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/core"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	sub_center_wrapper "github.com/BAN1ce/skyTree/internal/broker/subcenter/wrapper"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/kataras/go-events"
)

// Dependencies 显式列出 broker runtime 构建所需的上游依赖。
type Dependencies struct {
	Config              config.AppConfig
	Plugins             *plugin.Plugins
	Cluster             *clusterruntime.Runtime
	Stores              *storeruntime.Runtime
	StateCenters        *statecenter.Runtime
	LocalEvent          events.EventEmmiter
	ClientManager       *client.Manager
	ClientDeliveryEvent delivery_notify.ClientDeliveryEvent
}

// Runtime holds the broker core built from app-level dependencies.
type Runtime struct {
	BrokerCore *core.Broker
}

// Build creates the broker core from its upstream runtime dependencies.
func Build(deps Dependencies) (*Runtime, error) {
	if deps.Plugins == nil {
		return nil, fmt.Errorf("plugins are nil")
	}
	if deps.Cluster == nil {
		return nil, fmt.Errorf("cluster runtime is nil")
	}
	if deps.Stores == nil {
		return nil, fmt.Errorf("store runtime is nil")
	}
	if deps.StateCenters == nil {
		return nil, fmt.Errorf("state center runtime is nil")
	}

	willDelayCluster, err := clusterruntime.ClusterDescriptorByID(raft2.ClusterIDWillDelayCenter)
	if err != nil {
		return nil, fmt.Errorf("resolve will delay cluster descriptor failed: %w", err)
	}

	cfg := deps.Config
	var brokerCore *core.Broker
	stateRouter, err := staterouter.NewInProcessClient(staterouter.InProcessDependencies{
		SessionCenter:      deps.StateCenters.Session,
		SubscriptionCenter: sub_center_wrapper.NewSubCenterWrapper(deps.StateCenters.Subscription),
		NodeController:     deps.Cluster.NodeController,
		PublishRoute: func(ctx context.Context, req staterouter.RoutePublishRequest) error {
			if brokerCore == nil {
				return fmt.Errorf("broker core is nil")
			}
			return brokerCore.RoutePublish(ctx, req.Message)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create state router failed: %w", err)
	}

	options := []core.Option{
		core.WithNodeMeta(deps.Cluster.NodeMeta),
		core.WithGlobalNotify(deps.Cluster.NodeController),
		core.WithClientCenter(deps.StateCenters.Session),
		core.WithClusterController(deps.Cluster.ClusterState),
		core.WithRetainStore(retain.NewRetainStore(deps.Stores.KeyStore)),
		core.WithKeyStore(deps.Stores.KeyStore),
		core.WithPlugins(deps.Plugins),
		core.WithClientManager(deps.ClientManager),
		core.WithSubCenter(sub_center_wrapper.NewSubCenterWrapper(deps.StateCenters.Subscription)),
		core.WithStateRouter(stateRouter),
		core.WithEvent(deps.LocalEvent),
		core.WithDeliveryTaskStore(deps.Stores.DeliveryTaskStore),
		core.WithDeliveryCursorStore(deps.Stores.DeliveryCursorStore),
		core.WithClientDeliveryEvent(deps.ClientDeliveryEvent),
		core.WithWillDelayCenter(
			deps.StateCenters.WillDelay,
			deps.StateCenters.Session,
			deps.Cluster.Cluster,
			willDelayCluster.ClusterID,
			uint64(cfg.Cluster.LocalNodeID),
		),
	}
	if deps.Stores.SharedSubscriptionStore != nil {
		options = append(options, core.WithSharedSubscriptionStore(deps.Stores.SharedSubscriptionStore))
	}

	brokerCore, err = core.NewBroker(cfg.Broker, cfg.Plugins, cfg.Cluster, cfg.Delivery, options...)
	if err != nil {
		return nil, fmt.Errorf("create broker failed: %w", err)
	}

	return &Runtime{
		BrokerCore: brokerCore,
	}, nil
}
