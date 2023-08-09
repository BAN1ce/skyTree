package config

import (
	"fmt"
	"github.com/lni/dragonboat/v3/config"
	"path/filepath"
)

type Tree struct {
	DragonboatConfig config.Config
	NodeHostConfig   config.NodeHostConfig
	InitNode         map[uint64]string
}

func GetTree() Tree {
	var (
		initNode = map[uint64]string{
			1: "localhost:63001",
			2: "localhost:63002",
		}
	)
	var (
		nodeID      = *nodeIDFlag
		clusterID   = 123
		dataDir     = filepath.Join(fmt.Sprintf("snapshot/%d", nodeID))
		nodeAddress = initNode[nodeID]
	)
	return Tree{
		DragonboatConfig: config.Config{
			// ClusterID and NodeID of the raft node
			NodeID:    nodeID,
			ClusterID: uint64(clusterID),
			// In this example, we assume the end-to-end round trip time (RTT) between
			// NodeHost instances (on different machines, VMs or containers) are 200
			// millisecond, it is set in the RTTMillisecond field of the
			// config.NodeHostConfig instance below.
			// ElectionRTT is set to 10 in this example, it determines that the node
			// should start an election if there is no heartbeat from the leader for
			// 10 * RTT time intervals.
			ElectionRTT: 10,
			// HeartbeatRTT is set to 1 in this example, it determines that when the
			// node is a leader, it should broadcast heartbeat messages to its followers
			// every such 1 * RTT time interval.
			HeartbeatRTT: 1,
			CheckQuorum:  true,
			// SnapshotEntries determines how often should we take a snapshot of the
			// replicated state machine, it is set to 10 her which means a snapshot
			// will be captured for every 10 applied proposals (writes).
			// In your real world application, it should be set to much higher values
			// You need to determine a suitable value based on how much space you are
			// willing use on Raft Logs, how fast can you capture a snapshot of your
			// replicated state machine, how often such snapshot is going to be used
			// etc.
			SnapshotEntries: 10,
			// Once a snapshot is captured and saved, how many Raft entries already
			// covered by the new snapshot should be kept. This is useful when some
			// followers are just a little bit left behind, with such overhead Raft
			// entries, the leaders can send them regular entries rather than the full
			// snapshot image.
			CompactionOverhead: 5,
		},
		NodeHostConfig: config.NodeHostConfig{
			WALDir: dataDir,
			// NodeHostDir is where everything else is stored.
			NodeHostDir: dataDir,
			// RTTMillisecond is the average round trip time between NodeHosts (usually
			// on two machines/vms), it is in millisecond. Such RTT includes the
			// processing delays caused by NodeHosts, not just the network delay between
			// two NodeHost instances.
			RTTMillisecond: 200,
			// RaftAddress is used to identify the NodeHost instance
			RaftAddress: nodeAddress,
		},
		InitNode: initNode,
	}

}
