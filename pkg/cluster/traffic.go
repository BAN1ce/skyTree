package cluster

import (
	"sync"
	"time"
)

type TrafficState string

const (
	TrafficStateUnknown  TrafficState = "unknown"
	TrafficStateJoining  TrafficState = "joining"
	TrafficStateWarming  TrafficState = "warming"
	TrafficStateReady    TrafficState = "ready"
	TrafficStateSuspect  TrafficState = "suspect"
	TrafficStateDraining TrafficState = "draining"
)

type NodeTrafficStatus struct {
	State       TrafficState
	Reason      string
	UpdatedAt   time.Time
	Tracked     bool
	LastFailure time.Time
}

type TrafficStateStore interface {
	NodeTrafficState(nodeID uint64) TrafficState
	CanRouteToNode(nodeID uint64) bool
}

type TrafficController interface {
	TrafficStateStore
	SetNodeState(nodeID uint64, state TrafficState, reason string)
	NodeTrafficStatus(nodeID uint64) NodeTrafficStatus
}

type TrafficTracker struct {
	mu           sync.RWMutex
	statuses     map[uint64]NodeTrafficStatus
	routeUnknown bool
}

func NewTrafficTracker() *TrafficTracker {
	return &TrafficTracker{
		statuses:     make(map[uint64]NodeTrafficStatus, 16),
		routeUnknown: false,
	}
}

func (t *TrafficTracker) NodeTrafficState(nodeID uint64) TrafficState {
	status := t.NodeTrafficStatus(nodeID)
	if !status.Tracked {
		return TrafficStateUnknown
	}
	return status.State
}

func (t *TrafficTracker) NodeTrafficStatus(nodeID uint64) NodeTrafficStatus {
	if t == nil {
		return NodeTrafficStatus{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	status, ok := t.statuses[nodeID]
	if !ok {
		return NodeTrafficStatus{}
	}
	return status
}

func (t *TrafficTracker) CanRouteToNode(nodeID uint64) bool {
	if t == nil {
		return true
	}
	status := t.NodeTrafficStatus(nodeID)
	if !status.Tracked {
		return t.routeUnknown
	}
	return status.State == TrafficStateReady
}

func (t *TrafficTracker) SetNodeState(nodeID uint64, state TrafficState, reason string) {
	if t == nil || nodeID == 0 {
		return
	}
	now := time.Now()
	t.mu.Lock()
	status := t.statuses[nodeID]
	status.State = normalizeTrafficState(state)
	status.Reason = reason
	status.Tracked = true
	status.UpdatedAt = now
	if status.State == TrafficStateSuspect {
		status.LastFailure = now
	}
	t.statuses[nodeID] = status
	t.mu.Unlock()
}

func normalizeTrafficState(state TrafficState) TrafficState {
	switch state {
	case TrafficStateJoining, TrafficStateWarming, TrafficStateReady, TrafficStateSuspect, TrafficStateDraining:
		return state
	default:
		return TrafficStateUnknown
	}
}
