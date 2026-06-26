package consoleruntime

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	clusterpkg "github.com/BAN1ce/skyTree/pkg/cluster"
)

func TestProviderSummaryAndClientList(t *testing.T) {
	manager := brokerclient.NewManager()
	clientB := brokerclient.NewClient(nil)
	clientB.ID = "client-b"
	clientB.Username = "user-b"
	clientA := brokerclient.NewClient(nil)
	clientA.ID = "client-a"
	clientA.Username = "user-a"
	manager.AddClient(clientB.ID, clientB)
	manager.AddClient(clientA.ID, clientA)

	provider := NewProvider(Dependencies{
		Config: config.AppConfig{
			Server:  config.Server{Port: 9526},
			Storage: config.Store{Default: config.KeyStoreTypeBadger},
		},
		ClientManager: manager,
	})

	summary, err := provider.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.OnlineClients != 2 {
		t.Fatalf("expected two online clients, got %d", summary.OnlineClients)
	}
	if summary.ServerPort != 9526 {
		t.Fatalf("expected server port 9526, got %d", summary.ServerPort)
	}

	clients, err := provider.ListClients(context.Background(), api.ConsoleClientQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListClients failed: %v", err)
	}
	if clients.Total != 2 {
		t.Fatalf("expected total 2, got %d", clients.Total)
	}
	if got := clients.Items[0].ClientID; got != "client-a" {
		t.Fatalf("expected sorted first client client-a, got %q", got)
	}
}

func TestProviderListClusterNodesReturnsRaftRegisteredNodesWithoutControl(t *testing.T) {
	provider := NewProvider(Dependencies{
		ClusterState: fakeClusterState{
			nodes: []*clusterpkg.NodeMeta{
				{Cluster: config.Cluster{
					LocalNodeID:      2,
					LocalNodeAddress: "skytree-node2:63001",
					Join:             true,
					GRPC: config.GRPC{
						Addr:     "0.0.0.0:53001",
						Endpoint: "skytree-node2:53001",
					},
				}},
				{Cluster: config.Cluster{
					LocalNodeID:      1,
					LocalNodeAddress: "skytree-node1:63001",
					GRPC: config.GRPC{
						Addr:     "0.0.0.0:53001",
						Endpoint: "skytree-node1:53001",
					},
				}},
			},
		},
	})

	nodes, err := provider.ListClusterNodes(context.Background())
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if nodes.ControlAvailable {
		t.Fatalf("ControlAvailable = true, want false")
	}
	if len(nodes.RegisteredNodes) != 2 {
		t.Fatalf("len(RegisteredNodes) = %d, want 2", len(nodes.RegisteredNodes))
	}
	if nodes.RegisteredNodes[0].NodeID != 1 || nodes.RegisteredNodes[0].GRPCEndpoint != "skytree-node1:53001" {
		t.Fatalf("first registered node = %#v", nodes.RegisteredNodes[0])
	}
	if len(nodes.RuntimeNodes) != 0 {
		t.Fatalf("RuntimeNodes = %#v, want empty", nodes.RuntimeNodes)
	}
}

func TestProviderListClusterNodesMergesRuntimeNodesWhenControlEnabled(t *testing.T) {
	control := &fakeControlClient{
		nodes: []api.ConsoleRuntimeNode{
			{Name: "node-1", ServiceName: "skytree-node1", Status: "running"},
		},
	}
	provider := NewProvider(Dependencies{
		ClusterState: fakeClusterState{},
		Control:      control,
	})

	nodes, err := provider.ListClusterNodes(context.Background())
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if !nodes.ControlAvailable {
		t.Fatalf("ControlAvailable = false, want true")
	}
	if len(nodes.RuntimeNodes) != 1 || nodes.RuntimeNodes[0].ServiceName != "skytree-node1" {
		t.Fatalf("RuntimeNodes = %#v", nodes.RuntimeNodes)
	}
}

func TestProviderListClusterNodesReturnsCandidateNodesWithJoinState(t *testing.T) {
	control := &fakeControlClient{
		candidates: []api.ConsoleCandidateNode{
			{NodeID: 1, PodName: "skytree-0", JoinEligible: true},
			{NodeID: 4, PodName: "skytree-3", JoinEligible: true},
		},
	}
	provider := NewProvider(Dependencies{
		ClusterState: fakeClusterState{
			nodes: []*clusterpkg.NodeMeta{
				{Cluster: config.Cluster{LocalNodeID: 1}},
			},
		},
		Control: control,
	})

	nodes, err := provider.ListClusterNodes(context.Background())
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(nodes.CandidateNodes) != 2 {
		t.Fatalf("CandidateNodes = %#v, want 2 nodes", nodes.CandidateNodes)
	}
	if !nodes.CandidateNodes[0].Joined {
		t.Fatalf("first candidate should be joined")
	}
	if nodes.CandidateNodes[1].Joined {
		t.Fatalf("second candidate should not be joined")
	}
}

type fakeClusterState struct {
	nodes []*clusterpkg.NodeMeta
}

func (s fakeClusterState) AddNode(context.Context, *clusterpkg.NodeMeta) error {
	return nil
}

func (s fakeClusterState) RemoveNode(context.Context, uint64) error {
	return nil
}

func (s fakeClusterState) ListNode(context.Context) ([]*clusterpkg.NodeMeta, error) {
	return append([]*clusterpkg.NodeMeta(nil), s.nodes...), nil
}

type fakeControlClient struct {
	nodes      []api.ConsoleRuntimeNode
	candidates []api.ConsoleCandidateNode
	lastNode   string
	lastAction string
}

func (c *fakeControlClient) ListClusterNodes(context.Context) ([]api.ConsoleRuntimeNode, error) {
	return append([]api.ConsoleRuntimeNode(nil), c.nodes...), nil
}

func (c *fakeControlClient) ListCandidateNodes(context.Context) ([]api.ConsoleCandidateNode, error) {
	return append([]api.ConsoleCandidateNode(nil), c.candidates...), nil
}

func (c *fakeControlClient) RunClusterNodeAction(
	_ context.Context,
	node string,
	action string,
) (*api.ConsoleClusterNodeActionResult, error) {
	c.lastNode = node
	c.lastAction = action
	return &api.ConsoleClusterNodeActionResult{
		Node:   node,
		Action: action,
		Status: "accepted",
	}, nil
}
