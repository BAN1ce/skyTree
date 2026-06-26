package serverruntime

import (
	"fmt"
	"strings"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/internal/app/brokerruntime"
	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/internal/app/consoleruntime"
	"github.com/BAN1ce/skyTree/internal/app/lifecycle"
	"github.com/BAN1ce/skyTree/internal/app/statecenter"
	"github.com/BAN1ce/skyTree/internal/app/storeruntime"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	sub_center_wrapper "github.com/BAN1ce/skyTree/internal/broker/subcenter/wrapper"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	inner_grpc "github.com/BAN1ce/skyTree/internal/grpc"
	cluster_pkg "github.com/BAN1ce/skyTree/pkg/cluster"
	raft_pkg "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
)

// Dependencies 显式列出 server runtime 构建 API 和 gRPC 组件所需的依赖。
type Dependencies struct {
	Config           config.AppConfig
	Broker           *brokerruntime.Runtime
	Cluster          *clusterruntime.Runtime
	Stores           *storeruntime.Runtime
	StateCenters     *statecenter.Runtime
	HealthChecker    *inner_cluster.HealthChecker
	LocalEventCenter *eventbus.EventCenter[*delivery_event.Notify]
	ClientManager    *client.Manager
}

// Runtime 保存对外服务组件，交由 lifecycle runner 统一托管。
type Runtime struct {
	API  lifecycle.Component
	GRPC lifecycle.Component
}

// Build 创建 API server 和集群 gRPC server 组件。
func Build(deps Dependencies) (*Runtime, error) {
	if deps.Broker == nil || deps.Broker.BrokerCore == nil {
		return nil, fmt.Errorf("broker runtime is nil")
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

	cfg := deps.Config
	clusterOverview := clusterruntime.NewOverviewProvider(
		cfg,
		deps.Cluster.Cluster,
		deps.HealthChecker,
		deps.Stores.ClientDeliveryStore,
	)
	consoleControl := newConsoleControlClient(cfg, deps.Cluster)
	apiComponent := api.NewAPIWithConfig(
		fmt.Sprintf(":%d", cfg.Server.Port),
		&api.Component{
			ACLManager:       deps.Broker.BrokerCore.ACLManager(),
			ACLAdminUsername: cfg.Plugins.ACL.AdminUsername,
			ACLAdminPassword: cfg.Plugins.ACL.AdminPassword,
			ClusterHealth:    deps.HealthChecker,
			ClusterOverview:  clusterOverview,
			Console: consoleruntime.NewProvider(consoleruntime.Dependencies{
				Config:          cfg,
				ClientManager:   deps.ClientManager,
				StateCenters:    deps.StateCenters,
				Stores:          deps.Stores,
				ClusterOverview: clusterOverview,
				ClusterState:    deps.Cluster.ClusterState,
				Control:         consoleControl,
			}),
			ConsoleEnabled:  cfg.Console.Enabled,
			ConsoleUsername: cfg.Console.Username,
			ConsolePassword: cfg.Console.Password,
		},
		cfg.Server.TLS,
		cfg.Logging,
	)
	grpcComponent := inner_grpc.NewServer(
		cfg.Cluster.GRPC.Addr,
		deps.LocalEventCenter,
		deps.ClientManager,
		sub_center_wrapper.NewSubCenterWrapper(deps.StateCenters.Subscription),
		deps.StateCenters.Session,
		deps.Broker.BrokerCore.SharedSubscriptionManager,
		uint64(cfg.Cluster.LocalNodeID),
		cfg.Cluster.GRPC.TLS,
		deps.Cluster.GRPCAllowInsecure,
	)

	return &Runtime{
		API:  apiComponent,
		GRPC: grpcComponent,
	}, nil
}

func newConsoleControlClient(
	cfg config.AppConfig,
	clusterRuntime *clusterruntime.Runtime,
) consoleruntime.ControlClient {
	control := cfg.Console.Control
	if !control.Enabled {
		return nil
	}
	switch strings.TrimSpace(control.Provider) {
	case "k8s":
		var clusterState cluster_pkg.State
		var cluster *raft_pkg.Cluster
		if clusterRuntime != nil {
			clusterState = clusterRuntime.ClusterState
			cluster = clusterRuntime.Cluster
		}
		return consoleruntime.NewK8sControlClient(consoleruntime.K8sControlDependencies{
			Control:      control,
			BaseCluster:  cfg.Cluster,
			ClusterState: clusterState,
			Membership:   raft_pkg.NewMembershipManager(cluster, control.K8s.JoinTimeout),
			Traffic:      clusterRuntime.Traffic,
		})
	default:
		return consoleruntime.NewGardenerControlClient(control)
	}
}
