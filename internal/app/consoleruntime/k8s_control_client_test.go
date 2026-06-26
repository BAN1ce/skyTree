package consoleruntime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
)

func TestK8sControlClientListCandidateNodesDiscoversPods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/skytree-local/pods" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("labelSelector"); got != "app=skytree" {
			t.Fatalf("labelSelector = %q, want app=skytree", got)
		}
		err := json.NewEncoder(w).Encode(k8sPodList{
			Items: []k8sPod{
				{
					Metadata: k8sObjectMeta{
						Name:      "skytree-3",
						Namespace: "skytree-local",
					},
					Status: k8sPodStatus{
						Phase: "Running",
						PodIP: "10.244.0.13",
						Conditions: []k8sPodCondition{
							{Type: "Ready", Status: "True"},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewK8sControlClient(K8sControlDependencies{
		Control: config.ConsoleControl{
			K8s: config.ConsoleK8sControl{
				APIServer:      server.URL,
				Namespace:      "skytree-local",
				LabelSelector:  "app=skytree",
				RaftPort:       8080,
				GRPCPort:       8091,
				RequestTimeout: time.Second,
			},
		},
	})

	nodes, err := client.ListCandidateNodes(nil)
	if err != nil {
		t.Fatalf("ListCandidateNodes() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	node := nodes[0]
	if node.NodeID != 4 {
		t.Fatalf("NodeID = %d, want 4", node.NodeID)
	}
	if node.RaftAddress != "skytree-3.skytree-headless.skytree-local.svc.cluster.local:8080" {
		t.Fatalf("RaftAddress = %q", node.RaftAddress)
	}
	if node.GRPCEndpoint != "skytree-3.skytree-headless.skytree-local.svc.cluster.local:8091" {
		t.Fatalf("GRPCEndpoint = %q", node.GRPCEndpoint)
	}
	if !node.JoinEligible {
		t.Fatalf("JoinEligible = false, reason = %q", node.Reason)
	}
}
