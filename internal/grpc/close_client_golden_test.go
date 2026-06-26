package grpc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestCloseClientGoldenSnapshot(t *testing.T) {
	logger.LoadForTest()

	tests := []struct {
		name   string
		golden string
		req    *nodepb.CloseClientRequest
		setup  func(t *testing.T, mgr *client.Manager)
	}{
		{
			name:   "success_force_close_without_token",
			golden: "close_client_success.golden.json",
			req: &nodepb.CloseClientRequest{
				ClientID:   "client-success",
				OwnerToken: "",
			},
			setup: func(t *testing.T, mgr *client.Manager) {
				addManagedClient(t, mgr, "client-success")
			},
		},
		{
			name:   "fencing_owner_token_mismatch",
			golden: "close_client_owner_token_mismatch.golden.json",
			req: &nodepb.CloseClientRequest{
				ClientID:   "client-mismatch",
				OwnerToken: "request-token",
			},
			setup: func(t *testing.T, mgr *client.Manager) {
				addManagedClient(t, mgr, "client-mismatch")
			},
		},
		{
			name:   "client_not_found",
			golden: "close_client_not_found.golden.json",
			req: &nodepb.CloseClientRequest{
				ClientID:   "missing-client",
				OwnerToken: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := client.NewManager()
			if tt.setup != nil {
				tt.setup(t, mgr)
			}
			svc := NewServiceCloseClient(mgr)

			resp, err := svc.CloseClient(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("CloseClient returned error: %v", err)
			}

			actual := marshalResponseForGolden(t, resp)
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
			// Compare semantically rather than byte-for-byte: protojson deliberately
			// emits unstable whitespace, so a raw string diff is fragile across
			// protobuf library versions. Unmarshal both sides and compare the messages.
			if !goldenResponsesEqual(t, actual, expected) {
				t.Fatalf("snapshot mismatch for %s\nactual:\n%s\nexpected:\n%s", tt.name, actual, expected)
			}
		})
	}
}

func TestCloseClientRecordsSemanticMetrics(t *testing.T) {
	logger.LoadForTest()

	ownerConflictBefore := testutil.ToFloat64(metric.OwnerTokenConflictsTotal.WithLabelValues("remote_close", "skip_close"))
	ownerFailureBefore := testutil.ToFloat64(metric.RemoteCloseFailuresTotal.WithLabelValues("owner_conflict"))
	notFoundBefore := testutil.ToFloat64(metric.RemoteCloseFailuresTotal.WithLabelValues("not_found"))
	readClientBefore := histogramSampleCount(t, metric.CloseClientServerStageDurationSeconds, map[string]string{
		"stage":  "read_client",
		"result": "owner_conflict",
	})
	notFoundStageBefore := histogramSampleCount(t, metric.CloseClientServerStageDurationSeconds, map[string]string{
		"stage":  "total",
		"result": "not_found",
	})

	mgr := client.NewManager()
	addManagedClient(t, mgr, "client-mismatch")
	svc := NewServiceCloseClient(mgr)

	if _, err := svc.CloseClient(context.Background(), &nodepb.CloseClientRequest{
		ClientID:   "client-mismatch",
		OwnerToken: "request-token",
	}); err != nil {
		t.Fatalf("CloseClient mismatch returned error: %v", err)
	}
	if _, err := svc.CloseClient(context.Background(), &nodepb.CloseClientRequest{
		ClientID: "missing-client",
	}); err != nil {
		t.Fatalf("CloseClient missing returned error: %v", err)
	}

	if got := testutil.ToFloat64(metric.OwnerTokenConflictsTotal.WithLabelValues("remote_close", "skip_close")); got != ownerConflictBefore+1 {
		t.Fatalf("owner conflict counter = %v, want %v", got, ownerConflictBefore+1)
	}
	if got := testutil.ToFloat64(metric.RemoteCloseFailuresTotal.WithLabelValues("owner_conflict")); got != ownerFailureBefore+1 {
		t.Fatalf("remote close owner conflict counter = %v, want %v", got, ownerFailureBefore+1)
	}
	if got := testutil.ToFloat64(metric.RemoteCloseFailuresTotal.WithLabelValues("not_found")); got != notFoundBefore+1 {
		t.Fatalf("remote close not found counter = %v, want %v", got, notFoundBefore+1)
	}
	assertHistogramSampleCount(t, metric.CloseClientServerStageDurationSeconds, map[string]string{
		"stage":  "read_client",
		"result": "owner_conflict",
	}, readClientBefore+1)
	assertHistogramSampleCount(t, metric.CloseClientServerStageDurationSeconds, map[string]string{
		"stage":  "total",
		"result": "not_found",
	}, notFoundStageBefore+1)
}

func goldenResponsesEqual(t *testing.T, actual, expected []byte) bool {
	t.Helper()

	var gotMsg, wantMsg nodepb.CloseClientResponse
	if err := protojson.Unmarshal(actual, &gotMsg); err != nil {
		t.Fatalf("unmarshal actual response: %v", err)
	}
	if err := protojson.Unmarshal(expected, &wantMsg); err != nil {
		t.Fatalf("unmarshal golden response: %v", err)
	}
	return proto.Equal(&gotMsg, &wantMsg)
}

func marshalResponseForGolden(t *testing.T, resp *nodepb.CloseClientResponse) []byte {
	t.Helper()

	actual, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return append(actual, '\n')
}

func addManagedClient(t *testing.T, mgr *client.Manager, clientID string) {
	t.Helper()

	serverConn, peerConn := net.Pipe()
	t.Cleanup(func() {
		_ = serverConn.Close()
		_ = peerConn.Close()
	})

	c := client.NewClient(serverConn)
	c.ID = clientID
	mgr.AddClient(clientID, c)
}

func assertHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string, want uint64) {
	t.Helper()
	if got := histogramSampleCount(t, collector, labels); got != want {
		t.Fatalf("histogram sample count for labels %v = %d, want %d", labels, got, want)
	}
}

func histogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string) uint64 {
	t.Helper()
	metrics := make(chan prometheus.Metric, 16)
	go func() {
		collector.Collect(metrics)
		close(metrics)
	}()

	for item := range metrics {
		dtoMetric := &dto.Metric{}
		if err := item.Write(dtoMetric); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		if !metricLabelsMatch(dtoMetric, labels) {
			continue
		}
		if dtoMetric.Histogram == nil {
			return 0
		}
		return dtoMetric.Histogram.GetSampleCount()
	}
	return 0
}

func metricLabelsMatch(item *dto.Metric, labels map[string]string) bool {
	if item == nil {
		return false
	}
	got := make(map[string]string, len(item.Label))
	for _, pair := range item.Label {
		got[pair.GetName()] = pair.GetValue()
	}
	for name, wantValue := range labels {
		if got[name] != wantValue {
			return false
		}
	}
	return true
}
