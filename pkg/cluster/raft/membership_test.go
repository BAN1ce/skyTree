package raft

import (
	"context"
	"testing"
	"time"

	"github.com/lni/dragonboat/v3"
)

func TestMembershipManagerAddNodeSkipsExistingMembership(t *testing.T) {
	fake := &fakeMembershipNodeHost{
		memberships: map[uint64]*dragonboat.Membership{
			10: {
				Nodes:          map[uint64]string{1: "node-1:8080", 4: "node-4:8080"},
				ConfigChangeID: 7,
			},
			11: {
				Nodes:          map[uint64]string{1: "node-1:8080"},
				ConfigChangeID: 9,
			},
		},
	}
	manager := &MembershipManager{
		node:    fake,
		timeout: time.Second,
		descriptors: []ClusterDescriptor{
			{ClusterID: 10, Name: "alpha"},
			{ClusterID: 11, Name: "beta"},
		},
	}

	result, err := manager.AddNode(context.Background(), 4, "node-4:8080")
	if err != nil {
		t.Fatalf("AddNode() error = %v", err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(result.Groups))
	}
	if result.Groups[0].Status != MembershipStatusAlreadyJoined {
		t.Fatalf("first group status = %q", result.Groups[0].Status)
	}
	if result.Groups[1].Status != MembershipStatusJoined {
		t.Fatalf("second group status = %q", result.Groups[1].Status)
	}
	if len(fake.addCalls) != 1 {
		t.Fatalf("add calls = %#v, want one call", fake.addCalls)
	}
	call := fake.addCalls[0]
	if call.clusterID != 11 || call.nodeID != 4 || call.target != "node-4:8080" || call.configChangeID != 9 {
		t.Fatalf("unexpected add call: %#v", call)
	}
}

type fakeMembershipNodeHost struct {
	memberships map[uint64]*dragonboat.Membership
	addCalls    []fakeAddNodeCall
}

type fakeAddNodeCall struct {
	clusterID      uint64
	nodeID         uint64
	target         string
	configChangeID uint64
}

func (f *fakeMembershipNodeHost) SyncGetClusterMembership(
	_ context.Context,
	clusterID uint64,
) (*dragonboat.Membership, error) {
	return f.memberships[clusterID], nil
}

func (f *fakeMembershipNodeHost) SyncRequestAddNode(
	_ context.Context,
	clusterID uint64,
	nodeID uint64,
	target string,
	configChangeID uint64,
) error {
	f.addCalls = append(f.addCalls, fakeAddNodeCall{
		clusterID:      clusterID,
		nodeID:         nodeID,
		target:         target,
		configChangeID: configChangeID,
	})
	return nil
}
