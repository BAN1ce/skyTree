package clusterruntime

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/config"
	grpc_client "github.com/BAN1ce/skyTree/pkg/brokerapi/grpc"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	cluster_pkg "github.com/BAN1ce/skyTree/pkg/cluster"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

// Runtime 保存集群相关的运行时对象和节点控制依赖。
type Runtime struct {
	Cluster           *raft2.Cluster
	NodeMeta          *cluster_pkg.NodeMeta
	ClusterState      cluster_pkg.State
	Traffic           *cluster_pkg.TrafficTracker
	RaftGRPCClient    *grpc_client.RaftGRPCClient
	NodeController    cluster_pkg.NodeController
	GRPCAllowInsecure bool
}

// Start 根据集群配置启动 raft cluster，并返回集群运行时。
func Start(ctx context.Context, cfg config.AppConfig) (*Runtime, error) {
	clusterConfig := cfg.Cluster
	runtime := &Runtime{
		NodeMeta:          &cluster_pkg.NodeMeta{Cluster: clusterConfig},
		GRPCAllowInsecure: !clusterConfig.Enable || clusterConfig.GRPC.AllowInsecure,
	}

	if !clusterConfig.Enable {
		return runtime, nil
	}

	cluster := NewRaftCluster(clusterConfig)
	if err := cluster.Start(ctx); err != nil {
		return nil, fmt.Errorf("start cluster failed: %w", err)
	}
	runtime.Cluster = cluster
	return runtime, nil
}

// BuildClients 在 keystore 可用后创建集群状态和 raft gRPC 客户端。
func (r *Runtime) BuildClients(cfg config.AppConfig, keyStore store.KeyStore) {
	r.ClusterState = cluster_pkg.NewStateImpl(keyStore)
	r.Traffic = cluster_pkg.NewTrafficTracker()
	r.Traffic.SetNodeState(cfg.Cluster.LocalNodeID, cluster_pkg.TrafficStateReady, "")
	r.RaftGRPCClient = grpc_client.NewRaftGRPCClient(
		cfg.Cluster.LocalNodeID,
		r.ClusterState,
		cfg.Cluster.GRPC.TLS,
		r.GRPCAllowInsecure,
	)
	r.RaftGRPCClient.SetTrafficStateStore(r.Traffic)
	r.NodeController = cluster_pkg.NodeController(r.RaftGRPCClient)
}
