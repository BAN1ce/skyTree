package clusterruntime

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

// overviewProvider 为 API 层提供集群概览数据。
type overviewProvider struct {
	clusterEnabled bool
	cluster        *raft2.Cluster
	health         api.ClusterHealthProvider

	deliverySpec    config.ClientDeliverySpec
	deliverySpecErr error
	backlogStore    store.DeliveryBacklogSummaryStore
}

// NewOverviewProvider 创建集群概览 provider。
func NewOverviewProvider(
	cfg config.AppConfig,
	cluster *raft2.Cluster,
	healthChecker api.ClusterHealthProvider,
	clientDeliveryStore store.ClientDeliveryStore,
) api.ClusterOverviewProvider {
	spec, specErr := cfg.ResolveClientDeliverySpec()

	var backlogStore store.DeliveryBacklogSummaryStore
	if summaryStore, ok := clientDeliveryStore.(store.DeliveryBacklogSummaryStore); ok {
		backlogStore = summaryStore
	}

	return &overviewProvider{
		clusterEnabled:  cfg.Cluster.Enable,
		cluster:         cluster,
		health:          healthChecker,
		deliverySpec:    spec,
		deliverySpecErr: specErr,
		backlogStore:    backlogStore,
	}
}

// GetClusterOverview 汇总 raft group 状态和投递积压状态。
func (p *overviewProvider) GetClusterOverview(ctx context.Context) (*api.ClusterOverview, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	overview := &api.ClusterOverview{
		Timestamp:      time.Now(),
		ClusterEnabled: p.clusterEnabled,
	}
	overview.RaftGroups = p.buildRaftGroups()
	overview.DeliveryBacklog = p.buildDeliveryBacklog(ctx)
	return overview, nil
}

// buildRaftGroups 构建当前节点可观测的 raft group 概览。
func (p *overviewProvider) buildRaftGroups() []api.ClusterRaftGroupOverview {
	if !p.clusterEnabled || p.cluster == nil {
		return nil
	}

	descriptors := make([]raft2.ClusterDescriptor, 0, len(raft2.SystemClusterDescriptors()))
	descriptors = append(descriptors, raft2.SystemClusterDescriptors()...)

	raftGroups := make([]api.ClusterRaftGroupOverview, 0, len(descriptors))
	for _, descriptor := range descriptors {
		group := api.ClusterRaftGroupOverview{
			ClusterID:   descriptor.ClusterID,
			ClusterName: descriptor.Name,
			Kind:        string(descriptor.Kind),
			Health:      "unknown",
		}

		leaderNodeID, hasLeader, err := p.cluster.GetLeader(descriptor.ClusterID)
		if err == nil && hasLeader && leaderNodeID != 0 {
			group.HasLeader = true
			group.LeaderNodeID = leaderNodeID
		}

		group.Health = p.healthStatus(descriptor.ClusterID)
		group.ReplicationStatus = p.replicationStatus(group.HasLeader, group.Health)
		raftGroups = append(raftGroups, group)
	}

	sort.Slice(raftGroups, func(i, j int) bool {
		return raftGroups[i].ClusterID < raftGroups[j].ClusterID
	})
	return raftGroups
}

// healthStatus 将健康检查状态转换为 API 输出字符串。
func (p *overviewProvider) healthStatus(clusterID uint64) string {
	if p.health == nil {
		return "unknown"
	}
	status, ok := p.health.GetHealthStatus(clusterID)
	if !ok || status == nil {
		return "unknown"
	}
	switch status.Status {
	case inner_cluster.HealthStatusHealthy:
		return "healthy"
	case inner_cluster.HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// replicationStatus 根据 leader 和健康状态推导复制状态。
func (p *overviewProvider) replicationStatus(hasLeader bool, health string) string {
	if health == "unknown" {
		return "unknown"
	}
	if hasLeader && health == "healthy" {
		return "healthy"
	}
	return "degraded"
}

// buildDeliveryBacklog 构建客户端投递队列积压概览。
func (p *overviewProvider) buildDeliveryBacklog(ctx context.Context) api.ClusterDeliveryBacklogOverview {
	unsupported := func(reason string) api.ClusterDeliveryBacklogOverview {
		return api.ClusterDeliveryBacklogOverview{
			Supported: false,
			Reason:    reason,
		}
	}

	if p.deliverySpecErr != nil {
		return unsupported(fmt.Sprintf("resolve delivery spec failed: %v", p.deliverySpecErr))
	}

	switch p.deliverySpec.QueueType {
	case config.ClientDeliveryQueueTypeSingleNodeBadger:
		if p.backlogStore == nil {
			return unsupported("delivery queue store does not implement backlog summary")
		}
		summary, err := p.backlogStore.DeliveryBacklogSummary(ctx)
		if err != nil {
			return api.ClusterDeliveryBacklogOverview{
				Supported:     false,
				Reason:        err.Error(),
				PendingTasks:  summary.PendingTasks,
				ActiveClients: summary.ActiveClients,
			}
		}
		return api.ClusterDeliveryBacklogOverview{
			Supported:     true,
			PendingTasks:  summary.PendingTasks,
			ActiveClients: summary.ActiveClients,
		}
	case config.ClientDeliveryQueueTypeScylla:
		return unsupported("delivery queue type \"scylla\" backlog summary is not implemented")
	default:
		return unsupported(fmt.Sprintf("unsupported delivery queue type %q", p.deliverySpec.QueueType))
	}
}
