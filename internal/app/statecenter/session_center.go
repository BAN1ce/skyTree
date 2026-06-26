package statecenter

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/config"
	sc_raft "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/raft"
	sc_sm "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/statemachine"
	sc_wal "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/wal"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/lni/dragonboat/v3/statemachine"
)

// BuildSessionCenter 根据运行模式构建会话状态中心。
func BuildSessionCenter(
	ctx context.Context,
	cfg config.AppConfig,
	cluster *raft2.Cluster,
) (session.Center, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buildCenter(cfg, cluster, centerModeSpec[session.Center]{
		Name:          "session",
		LocalStateDir: "session_center",
		LocalWALBuilder: func(
			baseDir string,
			snapshotInterval time.Duration,
			snapshotEntries uint64,
		) (session.Center, error) {
			return sc_wal.NewLocalWALCenter(baseDir, snapshotInterval, snapshotEntries)
		},
		ClusterID: raft2.ClusterIDSessionCenter,
		StateMachine: func() statemachine.IStateMachine {
			return sc_sm.NewStateMachine()
		},
		RaftCenterBuild: func(
			clusterID uint64,
			cluster *raft2.Cluster,
			clusterCfg config.Cluster,
		) session.Center {
			return sc_raft.NewCluster(
				clusterCfg.LocalNodeID,
				clusterruntime.NewBusinessRaftClient(clusterID, cluster, clusterCfg),
			)
		},
	})
}
