package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

func TestClusterRoutesGoldenSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	fixed := time.Unix(1_700_000_000, 0).UTC()
	a := NewAPI(":0", &Component{
		ClusterHealth: fakeClusterHealthProvider{
			all: map[uint64]*inner_cluster.ClusterHealthInfo{
				2: {
					ClusterID:    2,
					ClusterName:  "key_store",
					Status:       inner_cluster.HealthStatusHealthy,
					LastCheck:    fixed,
					LastSuccess:  fixed,
					FailureCount: 0,
				},
				4: {
					ClusterID:    4,
					ClusterName:  "will_delay",
					Status:       inner_cluster.HealthStatusUnhealthy,
					LastCheck:    fixed,
					LastSuccess:  fixed.Add(-time.Minute),
					FailureCount: 3,
					Error:        "timeout",
				},
			},
		},
	})
	a.httpServer = gin.New()
	a.route()

	tests := []struct {
		name      string
		path      string
		golden    string
		normalize func(map[string]interface{})
	}{
		{
			name:   "summary",
			path:   "/api/v1/cluster/health",
			golden: "cluster_health_summary.golden.json",
			normalize: func(m map[string]interface{}) {
				normalizeClusterTimeFields(m)
			},
		},
		{
			name:   "detail",
			path:   "/api/v1/cluster/health/4",
			golden: "cluster_health_detail.golden.json",
			normalize: func(m map[string]interface{}) {
				normalizeClusterTimeFields(m)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			a.httpServer.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			tt.normalize(payload)
			actual, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				t.Fatalf("marshal normalized response: %v", err)
			}
			actual = append(actual, '\n')

			goldenPath := filepath.Join("testdata", tt.golden)
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(goldenPath, actual, 0o600); err != nil {
					t.Fatalf("write golden file: %v", err)
				}
			}

			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden file %s: %v", goldenPath, err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("snapshot mismatch for %s\nactual:\n%s\nexpected:\n%s", tt.name, actual, expected)
			}
		})
	}
}

func normalizeClusterTimeFields(payload map[string]interface{}) {
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		return
	}
	if _, ok := data["timestamp"]; ok {
		data["timestamp"] = "<time>"
	}
	if clusters, ok := data["clusters"].([]interface{}); ok {
		for _, item := range clusters {
			cluster, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if _, exists := cluster["last_check"]; exists {
				cluster["last_check"] = "<time>"
			}
			if _, exists := cluster["last_success"]; exists {
				cluster["last_success"] = "<time>"
			}
		}
	}
	if _, ok := data["last_check"]; ok {
		data["last_check"] = "<time>"
	}
	if _, ok := data["last_success"]; ok {
		data["last_success"] = "<time>"
	}
}
