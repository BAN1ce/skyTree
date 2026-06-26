package raft

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lni/dragonboat/v3"
)

const (
	MembershipStatusAlreadyJoined = "already_joined"
	MembershipStatusJoined        = "joined"
	MembershipStatusFailed        = "failed"
)

type MembershipResult struct {
	NodeID uint64
	Target string
	Groups []MembershipGroupResult
}

type MembershipGroupResult struct {
	ClusterID   uint64
	ClusterName string
	Status      string
	Error       string
}

type MembershipSnapshot struct {
	Groups []MembershipGroupSnapshot
}

type MembershipGroupSnapshot struct {
	ClusterID      uint64
	ClusterName    string
	Nodes          map[uint64]string
	Observers      map[uint64]string
	Witnesses      map[uint64]string
	Removed        map[uint64]struct{}
	ConfigChangeID uint64
}

type membershipNodeHost interface {
	SyncGetClusterMembership(ctx context.Context, clusterID uint64) (*dragonboat.Membership, error)
	SyncRequestAddNode(
		ctx context.Context,
		clusterID uint64,
		nodeID uint64,
		target string,
		configChangeID uint64,
	) error
}

type MembershipManager struct {
	node        membershipNodeHost
	timeout     time.Duration
	descriptors []ClusterDescriptor
}

func NewMembershipManager(cluster *Cluster, timeout time.Duration) *MembershipManager {
	var node membershipNodeHost
	if cluster != nil {
		node = cluster.node
	}
	return &MembershipManager{
		node:        node,
		timeout:     membershipTimeout(timeout),
		descriptors: SystemClusterDescriptors(),
	}
}

func (m *MembershipManager) AddNode(ctx context.Context, nodeID uint64, target string) (*MembershipResult, error) {
	if nodeID == 0 {
		return nil, errors.New("node id must be greater than 0")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("raft target is required")
	}
	if m == nil || m.node == nil {
		return nil, errors.New("raft membership manager is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result := &MembershipResult{
		NodeID: nodeID,
		Target: target,
		Groups: []MembershipGroupResult{},
	}
	var joinedErr error
	for _, descriptor := range m.clusterDescriptors() {
		groupResult, err := m.addNodeToGroup(ctx, descriptor, nodeID, target)
		result.Groups = append(result.Groups, groupResult)
		if err != nil {
			joinedErr = errors.Join(joinedErr, err)
		}
	}
	if joinedErr != nil {
		return result, joinedErr
	}
	return result, nil
}

func (m *MembershipManager) ListMembership(ctx context.Context) (*MembershipSnapshot, error) {
	if m == nil || m.node == nil {
		return nil, errors.New("raft membership manager is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot := &MembershipSnapshot{Groups: []MembershipGroupSnapshot{}}
	for _, descriptor := range m.clusterDescriptors() {
		membership, err := m.getMembership(ctx, descriptor.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("read raft membership: %w", err)
		}
		snapshot.Groups = append(snapshot.Groups, MembershipGroupSnapshot{
			ClusterID:      descriptor.ClusterID,
			ClusterName:    descriptor.Name,
			Nodes:          copyStringMap(membership.Nodes),
			Observers:      copyStringMap(membership.Observers),
			Witnesses:      copyStringMap(membership.Witnesses),
			Removed:        copySet(membership.Removed),
			ConfigChangeID: membership.ConfigChangeID,
		})
	}
	return snapshot, nil
}

func (m *MembershipManager) addNodeToGroup(
	ctx context.Context,
	descriptor ClusterDescriptor,
	nodeID uint64,
	target string,
) (MembershipGroupResult, error) {
	group := MembershipGroupResult{
		ClusterID:   descriptor.ClusterID,
		ClusterName: descriptor.Name,
	}
	membership, err := m.getMembership(ctx, descriptor.ClusterID)
	if err != nil {
		group.Status = MembershipStatusFailed
		group.Error = "read raft membership failed"
		return group, fmt.Errorf("read raft membership: %w", err)
	}
	if membershipContainsNode(membership, nodeID) {
		group.Status = MembershipStatusAlreadyJoined
		return group, nil
	}

	addCtx, cancel := context.WithTimeout(ctx, membershipTimeout(m.timeout))
	defer cancel()
	if err := m.node.SyncRequestAddNode(
		addCtx,
		descriptor.ClusterID,
		nodeID,
		target,
		membership.ConfigChangeID,
	); err != nil {
		group.Status = MembershipStatusFailed
		group.Error = "add raft node failed"
		return group, fmt.Errorf("add raft node: %w", err)
	}
	group.Status = MembershipStatusJoined
	return group, nil
}

func (m *MembershipManager) getMembership(ctx context.Context, clusterID uint64) (*dragonboat.Membership, error) {
	readCtx, cancel := context.WithTimeout(ctx, membershipTimeout(m.timeout))
	defer cancel()
	membership, err := m.node.SyncGetClusterMembership(readCtx, clusterID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, errors.New("raft membership is nil")
	}
	return membership, nil
}

func (m *MembershipManager) clusterDescriptors() []ClusterDescriptor {
	if m == nil || len(m.descriptors) == 0 {
		return SystemClusterDescriptors()
	}
	out := make([]ClusterDescriptor, len(m.descriptors))
	copy(out, m.descriptors)
	return out
}

func membershipTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return 5 * time.Second
}

func membershipContainsNode(membership *dragonboat.Membership, nodeID uint64) bool {
	if membership == nil {
		return false
	}
	if _, ok := membership.Nodes[nodeID]; ok {
		return true
	}
	if _, ok := membership.Observers[nodeID]; ok {
		return true
	}
	if _, ok := membership.Witnesses[nodeID]; ok {
		return true
	}
	return false
}

func copyStringMap(in map[uint64]string) map[uint64]string {
	out := make(map[uint64]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copySet(in map[uint64]struct{}) map[uint64]struct{} {
	out := make(map[uint64]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
