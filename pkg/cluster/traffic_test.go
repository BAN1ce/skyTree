package cluster

import "testing"

func TestTrafficTrackerRoutesTrackedReadyNodeOnly(t *testing.T) {
	tracker := NewTrafficTracker()

	if tracker.CanRouteToNode(9) {
		t.Fatal("untracked node should not be routeable before activation")
	}

	tracker.SetNodeState(9, TrafficStateJoining, "activation_pending")
	if tracker.CanRouteToNode(9) {
		t.Fatal("joining node should not be routeable")
	}

	tracker.SetNodeState(9, TrafficStateReady, "")
	if !tracker.CanRouteToNode(9) {
		t.Fatal("ready node should be routeable")
	}
}
