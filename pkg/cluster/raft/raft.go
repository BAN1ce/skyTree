package raft

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/statemachine"
)

const (
	ClusterIDSubCenter       uint64 = 0
	ClusterIDKeyStore        uint64 = 2
	ClusterIDSessionCenter   uint64 = 3
	ClusterIDWillDelayCenter uint64 = 4
)

type Cluster struct {
	initialMembers map[uint64]dragonboat.Target
	join           bool
	ctx            context.Context
	cancel         context.CancelFunc
	nodeConfig     config.NodeHostConfig
	state          sync.Map // map[config.Config]statemachine.IStateMachine
	node           *dragonboat.NodeHost
	closeOnce      sync.Once
}

func NewCluster(options ...Option) *Cluster {
	cluster := &Cluster{
		initialMembers: make(map[uint64]dragonboat.Target),
		join:           false,
		nodeConfig:     config.NodeHostConfig{},
		node:           nil,
	}
	for _, option := range options {
		option(cluster)
	}

	return cluster
}

func (c *Cluster) Start(ctx context.Context) error {
	var err error
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.node, err = dragonboat.NewNodeHost(c.nodeConfig)
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) Name() string {
	return "raft cluster"
}

func (c *Cluster) RegisterStateMachines(states map[config.Config]statemachine.IStateMachine) error {
	return c.RegisterStateMachinesWithMembers(states, nil)
}

// RegisterStateMachinesWithMembers starts state machines with per-cluster membership.
//
// membersByClusterID maps clusterID -> initial members. When a clusterID is present in the map,
// it overrides c.initialMembers for that specific cluster.
//
// For group-sharded workloads, each node must only start the Raft groups it owns.
// Otherwise every node will host every group and end up with full data replication (and high memory/FD usage).
func (c *Cluster) RegisterStateMachinesWithMembers(
	states map[config.Config]statemachine.IStateMachine,
	membersByClusterID map[uint64]map[uint64]dragonboat.Target,
) error {
	for cfg, state := range states {
		members := c.initialMembers
		if membersByClusterID != nil {
			if m, ok := membersByClusterID[cfg.ClusterID]; ok && len(m) != 0 {
				members = m
			}
		}

		// If this node is not a member of the cluster, skip starting it.
		// Non-member nodes must access the cluster via RPC/proxying to a member node.
		if !c.join {
			if _, ok := members[cfg.NodeID]; !ok {
				logger.Logger.Debug().
					Uint64("cluster_id", cfg.ClusterID).
					Uint64("node_id", cfg.NodeID).
					Msg("skip start raft cluster: local node is not a member")
				metric.SetRaftGroupStarted(ClusterName(cfg.ClusterID), cfg.ClusterID, false)
				continue
			}
		}

		c.state.Store(cfg, state)
		if err := c.node.StartCluster(members, c.join, func(_, _ uint64) statemachine.IStateMachine {
			return state
		}, cfg); err != nil {
			metric.RecordRaftGroupStart(ClusterName(cfg.ClusterID), cfg.ClusterID, err)
			logger.Logger.Error().Err(err).Msg("failed to start cluster")
			return err
		}
		metric.RecordRaftGroupStart(ClusterName(cfg.ClusterID), cfg.ClusterID, nil)

	}

	time.Sleep(1 * time.Second)

	return nil
}

func (c *Cluster) Close() error {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.node != nil {
			c.node.Stop()
			c.node = nil
		}
	})
	return nil
}

func (c *Cluster) GetLeader(clusterID uint64) (uint64, bool, error) {
	leaderNodeID, ok, err := c.node.GetLeaderID(clusterID)
	if err == nil && ok {
		metric.SetRaftGroupLeader(ClusterName(clusterID), clusterID, leaderNodeID)
	}
	return leaderNodeID, ok, err
}

func (c *Cluster) IsJoinMode() bool {
	if c == nil {
		return false
	}
	return c.join
}

// GetNodeHost 获取NodeHost实例
func (c *Cluster) GetNodeHost() *dragonboat.NodeHost {
	return c.node
}

func (c *Cluster) StartedClusterIDs() []uint64 {
	if c == nil {
		return nil
	}
	ids := []uint64{}
	c.state.Range(func(key, _ interface{}) bool {
		cfg, ok := key.(config.Config)
		if !ok {
			return true
		}
		ids = append(ids, cfg.ClusterID)
		return true
	})
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}
