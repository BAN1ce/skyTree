package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/brokerruntime"
	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/internal/app/lifecycle"
	"github.com/BAN1ce/skyTree/internal/app/serverruntime"
	"github.com/BAN1ce/skyTree/internal/app/statecenter"
	"github.com/BAN1ce/skyTree/internal/app/storeruntime"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
	"github.com/kataras/go-events"
)

const DefaultEventBusListenerQueueCapacity = 64

// EventDriverBinder 用于在应用构建时向本地事件总线绑定外部监听器。
type EventDriverBinder func(driver events.EventEmmiter)

// Config 保存 bootstrap 构建 AppRuntime 时需要的外部配置和策略。
type Config struct {
	Plugins              *plugin.Plugins
	EventDriverBinder    EventDriverBinder
	CriticalStartupGrace time.Duration
}

// BuildAppRuntime 按固定启动依赖顺序构建 App 运行时。
func BuildAppRuntime(ctx context.Context, cfg config.AppConfig, bootstrapCfg Config) (_ *AppRuntime, err error) {
	if err := validateConfig(bootstrapCfg); err != nil {
		return nil, err
	}

	var rollback rollbackStack
	defer func() {
		if err == nil {
			return
		}
		if closeErr := rollback.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	localEvent := events.New()
	if bootstrapCfg.EventDriverBinder != nil {
		bootstrapCfg.EventDriverBinder(localEvent)
	}

	clusterRuntime, err := clusterruntime.Start(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if clusterRuntime.Cluster != nil {
		rollback.Add(clusterRuntime.Cluster)
	}

	storeRuntime, err := storeruntime.Build(ctx, cfg, clusterRuntime.Cluster)
	if err != nil {
		return nil, err
	}
	resourceClosers := buildResourceClosers(clusterRuntime, storeRuntime)
	rollback = rollbackStack{}
	rollback.Add(resourceClosers...)

	clusterRuntime.BuildClients(cfg, storeRuntime.KeyStore)
	rollback.Add(clusterRuntime.RaftGRPCClient)

	localEventCenter := eventbus.NewEventCenter[*delivery_event.Notify](
		eventbus.WithListenerQueueCapacity(DefaultEventBusListenerQueueCapacity),
	)
	clientManager := client.NewManager()
	clientDeliveryEvent := delivery_notify.New(
		uint64(cfg.Cluster.LocalNodeID),
		localEventCenter,
		clusterRuntime.NodeController,
	)

	stateRuntime, err := statecenter.Build(ctx, cfg, clusterRuntime.Cluster)
	if err != nil {
		return nil, err
	}

	healthChecker := clusterruntime.BuildHealthChecker(ctx, cfg, clusterRuntime.Cluster, localEvent)
	if healthChecker != nil {
		clusterruntime.RegisterHealthCheckEventListeners(localEvent)
	}

	brokerRuntime, err := brokerruntime.Build(brokerruntime.Dependencies{
		Config:              cfg,
		Plugins:             bootstrapCfg.Plugins,
		Cluster:             clusterRuntime,
		Stores:              storeRuntime,
		StateCenters:        stateRuntime,
		LocalEvent:          localEvent,
		ClientManager:       clientManager,
		ClientDeliveryEvent: clientDeliveryEvent,
	})
	if err != nil {
		return nil, err
	}

	serverRuntime, err := serverruntime.Build(serverruntime.Dependencies{
		Config:           cfg,
		Broker:           brokerRuntime,
		Cluster:          clusterRuntime,
		Stores:           storeRuntime,
		StateCenters:     stateRuntime,
		HealthChecker:    healthChecker,
		LocalEventCenter: localEventCenter,
		ClientManager:    clientManager,
	})
	if err != nil {
		return nil, err
	}

	runner := lifecycle.NewRunner(ctx, buildManagedComponents(
		bootstrapCfg.CriticalStartupGrace,
		clusterRuntime,
		brokerRuntime,
		serverRuntime,
	))

	rollback = rollbackStack{}
	return &AppRuntime{
		LocalEvent:       localEvent,
		LocalEventCenter: localEventCenter,
		Cluster:          clusterRuntime,
		Stores:           storeRuntime,
		StateCenters:     stateRuntime,
		Broker:           brokerRuntime,
		Servers:          serverRuntime,
		HealthChecker:    healthChecker,
		Runner:           runner,
		resources:        resourceClosers,
	}, nil
}

// buildManagedComponents 将已构建的 runtime 转换为生命周期 runner 可管理的组件列表。
func buildManagedComponents(
	criticalGrace time.Duration,
	clusterRuntime *clusterruntime.Runtime,
	brokerRuntime *brokerruntime.Runtime,
	serverRuntime *serverruntime.Runtime,
) []lifecycle.ManagedComponent {
	return []lifecycle.ManagedComponent{
		{
			Component:    brokerRuntime.BrokerCore,
			Critical:     true,
			StartMode:    lifecycle.StartModeBlocking,
			StartupGrace: criticalGrace,
		},
		{
			Component:    serverRuntime.API,
			Critical:     true,
			StartMode:    lifecycle.StartModeBlocking,
			StartupGrace: criticalGrace,
		},
		{
			Component:    serverRuntime.GRPC,
			Critical:     true,
			StartMode:    lifecycle.StartModeBlocking,
			StartupGrace: criticalGrace,
		},
		{
			Component:    clusterRuntime.RaftGRPCClient,
			Critical:     false,
			StartMode:    lifecycle.StartModeBlocking,
			StartupGrace: criticalGrace,
		},
	}
}

// buildResourceClosers 收集 AppRuntime 拥有的底层资源关闭器。
func buildResourceClosers(clusterRuntime *clusterruntime.Runtime, storeRuntime *storeruntime.Runtime) []io.Closer {
	closers := []io.Closer{storeRuntime.KeyStore}
	if closer, ok := interface{}(storeRuntime.ClientDeliveryStore).(io.Closer); ok && closer != nil {
		closers = append(closers, closer)
	}
	if clusterRuntime.Cluster != nil {
		closers = append(closers, clusterRuntime.Cluster)
	}
	return closers
}

// validateConfig 校验 bootstrap 构建所需的最小配置。
func validateConfig(cfg Config) error {
	if cfg.Plugins == nil {
		return fmt.Errorf("plugins are nil")
	}
	return nil
}
