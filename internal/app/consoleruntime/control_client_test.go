package consoleruntime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
)

func TestGardenerControlClientSendsBearerTokenAndParsesNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cluster/nodes" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		_, _ = w.Write([]byte(`{"nodes":[]}`))
	}))
	defer server.Close()

	client := NewGardenerControlClient(config.ConsoleControl{
		BaseURL: server.URL,
		Token:   "secret",
		Timeout: time.Second,
	})
	nodes, err := client.ListClusterNodes(nil)
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if nodes == nil {
		t.Fatalf("nodes should be non-nil")
	}
}

func TestGardenerControlClientPostsNodeAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/cluster/nodes/node-1/actions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var req gardenerClusterActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Action != "restart" {
			t.Fatalf("action = %q, want restart", req.Action)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"node":   "node-1",
			"action": "restart",
			"status": "accepted",
		})
	}))
	defer server.Close()

	client := NewGardenerControlClient(config.ConsoleControl{
		BaseURL: server.URL,
		Timeout: time.Second,
	})
	result, err := client.RunClusterNodeAction(nil, "node-1", "restart")
	if err != nil {
		t.Fatalf("RunClusterNodeAction() error = %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("status = %q, want accepted", result.Status)
	}
}
