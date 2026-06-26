package core

import (
	"github.com/BAN1ce/skyTree/logger"
)

type raftLeaderReader interface {
	GetLeader(clusterID uint64) (uint64, bool, error)
}

func (b *Broker) isRaftGroupLeader(clusterID uint64) bool {
	if b == nil {
		return false
	}
	if !b.config.cluster.Enable {
		return true
	}
	return isRaftGroupLeaderForNode(b.cluster.raft, clusterID, b.localNodeID())
}

func (b *Broker) localNodeID() uint64 {
	if b == nil {
		return 0
	}
	if b.config.cluster.LocalNodeID != 0 {
		return b.config.cluster.LocalNodeID
	}
	if b.cluster.nodeMeta != nil {
		return b.cluster.nodeMeta.LocalNodeID
	}
	return 0
}

func isRaftGroupLeaderForNode(reader raftLeaderReader, clusterID uint64, localNodeID uint64) bool {
	if reader == nil || localNodeID == 0 {
		return false
	}
	leaderID, valid, err := reader.GetLeader(clusterID)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.Warn().Err(err).Uint64("cluster_id", clusterID).Msg("failed to get raft group leader")
		}
		return false
	}
	return valid && leaderID == localNodeID
}
