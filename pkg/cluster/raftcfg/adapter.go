package raftcfg

import (
	"fmt"
	"time"

	appcfg "github.com/BAN1ce/skyTree/config"
	dbconfig "github.com/lni/dragonboat/v3/config"
)

func InitialMembers(c appcfg.Cluster) map[uint64]string {
	return c.Member
}

func NodeHostConfig(c appcfg.Cluster) dbconfig.NodeHostConfig {
	return dbconfig.NodeHostConfig{
		WALDir:         fmt.Sprintf("%s_wal_%d", c.DataDir, c.LocalNodeID),
		NodeHostDir:    fmt.Sprintf("%s_%d", c.DataDir, c.LocalNodeID),
		RTTMillisecond: 10,
		RaftAddress:    c.LocalNodeAddress,
		EnableMetrics:  true,
	}
}

func RaftConfig(c appcfg.Cluster) dbconfig.Config {
	return dbconfig.Config{
		NodeID:             c.LocalNodeID,
		ElectionRTT:        200,
		HeartbeatRTT:       5,
		CheckQuorum:        true,
		SnapshotEntries:    200000,
		CompactionOverhead: 5000,
	}
}

func HealthCheckWithDefaults(h appcfg.HealthCheck) appcfg.HealthCheck {
	if h.Interval == 0 {
		h.Interval = 30 * time.Second
	}
	if h.Timeout == 0 {
		h.Timeout = 5 * time.Second
	}
	if h.MaxRetries == 0 {
		h.MaxRetries = 3
	}
	return h
}
