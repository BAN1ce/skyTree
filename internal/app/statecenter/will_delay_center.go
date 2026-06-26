package statecenter

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/config"
	will_delay2 "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	will_delay_raft "github.com/BAN1ce/skyTree/internal/broker/willdelay/raft"
	will_delay_sm "github.com/BAN1ce/skyTree/internal/broker/willdelay/statemachine"
	will_delay_wal "github.com/BAN1ce/skyTree/internal/broker/willdelay/wal"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/lni/dragonboat/v3/statemachine"
)

// BuildWillDelayCenter 根据运行模式构建 will-delay 状态中心。
func BuildWillDelayCenter(
	ctx context.Context,
	cfg config.AppConfig,
	cluster *raft2.Cluster,
) (will_delay2.Center, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buildCenter(cfg, cluster, centerModeSpec[will_delay2.Center]{
		Name:          "will delay",
		LocalStateDir: "will_delay_center",
		LocalWALBuilder: func(
			baseDir string,
			snapshotInterval time.Duration,
			snapshotEntries uint64,
		) (will_delay2.Center, error) {
			return will_delay_wal.NewLocalWALCenter(baseDir, snapshotInterval, snapshotEntries)
		},
		ClusterID: raft2.ClusterIDWillDelayCenter,
		StateMachine: func() statemachine.IStateMachine {
			return will_delay_sm.New()
		},
		RaftCenterBuild: func(
			clusterID uint64,
			cluster *raft2.Cluster,
			clusterCfg config.Cluster,
		) will_delay2.Center {
			return will_delay_raft.NewCluster(
				clusterCfg.LocalNodeID,
				clusterruntime.NewBusinessRaftClient(clusterID, cluster, clusterCfg),
			)
		},
	})
}
