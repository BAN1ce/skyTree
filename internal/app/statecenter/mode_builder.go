package statecenter

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/cluster/healthcheck"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/cluster/raftcfg"
	config2 "github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/statemachine"
)

// centerModeSpec 描述一个状态中心在本地模式和集群模式下的构建方式。
type centerModeSpec[T any] struct {
	Name            string
	LocalStateDir   string
	LocalWALBuilder func(baseDir string, snapshotInterval time.Duration, snapshotEntries uint64) (T, error)

	ClusterID       uint64
	StateMachine    func() statemachine.IStateMachine
	RaftCenterBuild func(clusterID uint64, cluster *raft2.Cluster, clusterCfg config.Cluster) T
}

// buildCenter 根据集群开关选择本地 WAL 或 raft 状态中心实现。
func buildCenter[T any](cfg config.AppConfig, cluster *raft2.Cluster, spec centerModeSpec[T]) (T, error) {
	var zero T

	if spec.LocalWALBuilder == nil || spec.StateMachine == nil || spec.RaftCenterBuild == nil {
		return zero, fmt.Errorf("%s center build spec is incomplete", spec.Name)
	}

	if !cfg.Cluster.Enable {
		baseDir := filepath.Join(cfg.Broker.LocalState.DataDir, spec.LocalStateDir)
		center, err := spec.LocalWALBuilder(
			baseDir,
			cfg.Broker.LocalState.SnapshotInterval,
			cfg.Broker.LocalState.SnapshotEntries,
		)
		if err != nil {
			return zero, fmt.Errorf("create local %s center failed: %w", spec.Name, err)
		}
		return center, nil
	}

	if cluster == nil {
		return zero, errors.New("cluster is nil in cluster mode")
	}

	clusterCfg := raftcfg.RaftConfig(cfg.Cluster)
	descriptor, err := clusterruntime.ClusterDescriptorByID(spec.ClusterID)
	if err != nil {
		return zero, fmt.Errorf("resolve %s cluster descriptor failed: %w", spec.Name, err)
	}
	clusterCfg.ClusterID = descriptor.ClusterID

	if err := cluster.RegisterStateMachines(map[config2.Config]statemachine.IStateMachine{
		clusterCfg: healthcheck.NewHealthCheckWrapper(spec.StateMachine()),
	}); err != nil {
		return zero, fmt.Errorf("register %s state machine failed: %w", spec.Name, err)
	}

	return spec.RaftCenterBuild(clusterCfg.ClusterID, cluster, cfg.Cluster), nil
}
