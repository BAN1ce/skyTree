package consoleruntime

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
	cluster_pkg "github.com/BAN1ce/skyTree/pkg/cluster"
	raft_pkg "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

type fakeMembershipJoiner struct {
	result *raft_pkg.MembershipResult
	err    error
}

func (f fakeMembershipJoiner) AddNode(context.Context, uint64, string) (*raft_pkg.MembershipResult, error) {
	return f.result, f.err
}

type recordingClusterState struct {
	added []*cluster_pkg.NodeMeta
}

func (s *recordingClusterState) AddNode(_ context.Context, node *cluster_pkg.NodeMeta) error {
	s.added = append(s.added, node)
	return nil
}

func (s *recordingClusterState) RemoveNode(context.Context, uint64) error {
	return nil
}

func (s *recordingClusterState) ListNode(context.Context) ([]*cluster_pkg.NodeMeta, error) {
	return nil, nil
}

type stubActivationProber struct {
	grpcErrs   []error
	healthErrs []error
	mqttErrs   []error
}

func (p *stubActivationProber) ProbeGRPC(context.Context, string, config.TLS, bool) error {
	return popProbeError(&p.grpcErrs)
}

func (p *stubActivationProber) ProbeHealth(context.Context, string) error {
	return popProbeError(&p.healthErrs)
}

func (p *stubActivationProber) ProbeMQTT(context.Context, string) error {
	return popProbeError(&p.mqttErrs)
}

func popProbeError(queue *[]error) error {
	if len(*queue) == 0 {
		return nil
	}
	err := (*queue)[0]
	*queue = (*queue)[1:]
	return err
}

func TestK8sControlClientJoinStartsActivationAndMarksReady(t *testing.T) {
	tracker := cluster_pkg.NewTrafficTracker()
	state := &recordingClusterState{}
	client := NewK8sControlClient(K8sControlDependencies{
		Control: config.ConsoleControl{
			K8s: config.ConsoleK8sControl{
				Namespace:      "skytree-local",
				ServiceName:    "skytree-headless",
				RaftPort:       8080,
				GRPCPort:       8091,
				RequestTimeout: time.Second,
				JoinTimeout:    time.Second,
			},
		},
		BaseCluster:  config.Cluster{GRPC: config.GRPC{AllowInsecure: true}},
		ClusterState: state,
		Membership: fakeMembershipJoiner{
			result: &raft_pkg.MembershipResult{},
		},
		Traffic: tracker,
	})
	client.activationProber = &stubActivationProber{}
	client.activationWarmup = 5 * time.Millisecond
	client.activationRetryInterval = 5 * time.Millisecond
	client.activationSuccessThreshold = 1
	client.listPodsFn = func(context.Context) ([]k8sPod, error) {
		return []k8sPod{
			{
				Metadata: k8sObjectMeta{Name: "skytree-1", Namespace: "skytree-local"},
				Status: k8sPodStatus{
					Phase: "Running",
					PodIP: "10.0.0.2",
					Conditions: []k8sPodCondition{
						{Type: "Ready", Status: "True"},
					},
				},
			},
		}, nil
	}

	result, err := client.RunClusterNodeAction(context.Background(), "skytree-1", "join")
	if err != nil {
		t.Fatalf("RunClusterNodeAction returned error: %v", err)
	}
	if result == nil || result.Status == "" {
		t.Fatalf("unexpected action result: %+v", result)
	}

	waitForTrafficState(t, tracker, 2, cluster_pkg.TrafficStateReady)
	if len(state.added) != 1 {
		t.Fatalf("AddNode calls = %d, want 1", len(state.added))
	}
}

func TestK8sControlClientJoinMarksSuspectWhenActivationFails(t *testing.T) {
	tracker := cluster_pkg.NewTrafficTracker()
	client := NewK8sControlClient(K8sControlDependencies{
		Control: config.ConsoleControl{
			K8s: config.ConsoleK8sControl{
				Namespace:      "skytree-local",
				ServiceName:    "skytree-headless",
				RaftPort:       8080,
				GRPCPort:       8091,
				RequestTimeout: time.Second,
				JoinTimeout:    30 * time.Millisecond,
			},
		},
		BaseCluster:  config.Cluster{GRPC: config.GRPC{AllowInsecure: true}},
		ClusterState: &recordingClusterState{},
		Membership: fakeMembershipJoiner{
			result: &raft_pkg.MembershipResult{},
		},
		Traffic: tracker,
	})
	client.activationProber = &stubActivationProber{
		mqttErrs: []error{
			context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded,
			context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded,
			context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded,
			context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded,
		},
	}
	client.activationWarmup = 5 * time.Millisecond
	client.activationRetryInterval = 5 * time.Millisecond
	client.activationSuccessThreshold = 1
	client.listPodsFn = func(context.Context) ([]k8sPod, error) {
		return []k8sPod{
			{
				Metadata: k8sObjectMeta{Name: "skytree-1", Namespace: "skytree-local"},
				Status: k8sPodStatus{
					Phase: "Running",
					PodIP: "10.0.0.2",
					Conditions: []k8sPodCondition{
						{Type: "Ready", Status: "True"},
					},
				},
			},
		}, nil
	}

	if _, err := client.RunClusterNodeAction(context.Background(), "skytree-1", "join"); err != nil {
		t.Fatalf("RunClusterNodeAction returned error: %v", err)
	}

	waitForTrafficState(t, tracker, 2, cluster_pkg.TrafficStateSuspect)
}

func waitForTrafficState(t *testing.T, tracker *cluster_pkg.TrafficTracker, nodeID uint64, want cluster_pkg.TrafficState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := tracker.NodeTrafficState(nodeID); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("node %d traffic state = %s, want %s", nodeID, tracker.NodeTrafficState(nodeID), want)
}

var _ = api.ConsoleCandidateNode{}
