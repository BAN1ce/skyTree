package core

import (
	"errors"
	"testing"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/cluster"
)

type fakeRaftLeaderReader struct {
	leaderID uint64
	valid    bool
	err      error
}

func (r fakeRaftLeaderReader) GetLeader(uint64) (uint64, bool, error) {
	return r.leaderID, r.valid, r.err
}

func TestBrokerIsRaftGroupLeaderTreatsSingleNodeAsLeader(t *testing.T) {
	b := &Broker{}

	if !b.isRaftGroupLeader(3) {
		t.Fatal("expected single-node broker to run leader-only tasks")
	}
}

func TestBrokerLocalNodeIDPrefersConfig(t *testing.T) {
	b := &Broker{
		config: brokerConfigSet{
			cluster: config.Cluster{LocalNodeID: 7},
		},
		cluster: brokerClusterResources{
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{LocalNodeID: 9}},
		},
	}

	if got := b.localNodeID(); got != 7 {
		t.Fatalf("localNodeID = %d, want 7", got)
	}
}

func TestBrokerLocalNodeIDFallsBackToNodeMeta(t *testing.T) {
	b := &Broker{
		cluster: brokerClusterResources{
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{LocalNodeID: 9}},
		},
	}

	if got := b.localNodeID(); got != 9 {
		t.Fatalf("localNodeID = %d, want 9", got)
	}
}

func TestIsRaftGroupLeaderForNode(t *testing.T) {
	tests := []struct {
		name        string
		reader      raftLeaderReader
		localNodeID uint64
		want        bool
	}{
		{
			name:        "leader match",
			reader:      fakeRaftLeaderReader{leaderID: 3, valid: true},
			localNodeID: 3,
			want:        true,
		},
		{
			name:        "leader mismatch",
			reader:      fakeRaftLeaderReader{leaderID: 4, valid: true},
			localNodeID: 3,
			want:        false,
		},
		{
			name:        "invalid leader",
			reader:      fakeRaftLeaderReader{leaderID: 3, valid: false},
			localNodeID: 3,
			want:        false,
		},
		{
			name:        "get leader error",
			reader:      fakeRaftLeaderReader{err: errors.New("leader unavailable")},
			localNodeID: 3,
			want:        false,
		},
		{
			name:        "missing node id",
			reader:      fakeRaftLeaderReader{leaderID: 3, valid: true},
			localNodeID: 0,
			want:        false,
		},
		{
			name:        "missing reader",
			reader:      nil,
			localNodeID: 3,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRaftGroupLeaderForNode(tt.reader, 3, tt.localNodeID)
			if got != tt.want {
				t.Fatalf("isRaftGroupLeaderForNode = %v, want %v", got, tt.want)
			}
		})
	}
}
