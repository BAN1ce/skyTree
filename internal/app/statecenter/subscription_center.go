package statecenter

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/config"
	sub_raft "github.com/BAN1ce/skyTree/internal/broker/subcenter/raft"
	sub_sm "github.com/BAN1ce/skyTree/internal/broker/subcenter/statemachine"
	sub_wal "github.com/BAN1ce/skyTree/internal/broker/subcenter/wal"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/lni/dragonboat/v3/statemachine"
)

// BuildSubscriptionCenter 根据运行模式构建订阅状态中心。
func BuildSubscriptionCenter(
	ctx context.Context,
	cfg config.AppConfig,
	cluster *raft2.Cluster,
) (subscription.Center, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buildCenter(cfg, cluster, centerModeSpec[subscription.Center]{
		Name:          "sub",
		LocalStateDir: "sub_center",
		LocalWALBuilder: func(
			baseDir string,
			snapshotInterval time.Duration,
			snapshotEntries uint64,
		) (subscription.Center, error) {
			return sub_wal.NewLocalWALCenter(baseDir, snapshotInterval, snapshotEntries)
		},
		ClusterID: raft2.ClusterIDSubCenter,
		StateMachine: func() statemachine.IStateMachine {
			return sub_sm.NewStateMachine()
		},
		RaftCenterBuild: func(
			clusterID uint64,
			cluster *raft2.Cluster,
			clusterCfg config.Cluster,
		) subscription.Center {
			return sub_raft.NewCluster(clusterruntime.NewBusinessRaftClient(clusterID, cluster, clusterCfg))
		},
	})
}
