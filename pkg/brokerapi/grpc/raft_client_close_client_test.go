package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type staticClusterState struct {
	nodes []*cluster.NodeMeta
}

func (s staticClusterState) AddNode(context.Context, *cluster.NodeMeta) error { return nil }
func (s staticClusterState) RemoveNode(context.Context, uint64) error         { return nil }
func (s staticClusterState) ListNode(context.Context) ([]*cluster.NodeMeta, error) {
	return s.nodes, nil
}

func TestRequestCloseClientSkipsLocalNodeAndRecordsPath(t *testing.T) {
	client := NewRaftGRPCClient(7, nil, config.TLS{Enabled: false}, true)

	pathBefore := testutil.ToFloat64(metric.CloseClientClientPathTotal.WithLabelValues("local_skip", "success"))
	totalBefore := histogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "total",
		"result": "success",
	})

	if err := client.RequestCloseClient(context.Background(), 7, "client-a", "owner-a"); err != nil {
		t.Fatalf("RequestCloseClient returned error: %v", err)
	}

	if got := testutil.ToFloat64(metric.CloseClientClientPathTotal.WithLabelValues("local_skip", "success")); got != pathBefore+1 {
		t.Fatalf("local skip path counter = %v, want %v", got, pathBefore+1)
	}
	assertHistogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "total",
		"result": "success",
	}, totalBefore+1)
}

func TestRequestCloseClientRecordsGetConnTimeoutMetrics(t *testing.T) {
	client := NewRaftGRPCClient(1, blockingClusterState{}, config.TLS{Enabled: false}, true)
	client.rpcTimeout = 20 * time.Millisecond

	stageBefore := histogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "get_conn_total",
		"result": "timeout",
	})
	failureBefore := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("get_conn_total", "deadline_exceeded"))

	err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a")
	if err == nil {
		t.Fatal("expected RequestCloseClient to fail")
	}

	assertHistogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "get_conn_total",
		"result": "timeout",
	}, stageBefore+1)
	if got := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("get_conn_total", "deadline_exceeded")); got != failureBefore+1 {
		t.Fatalf("get_conn timeout failure counter = %v, want %v", got, failureBefore+1)
	}
}

func TestRequestCloseClientRecordsRPCInvokeFailureMetrics(t *testing.T) {
	address, stop := startCloseClientTestServer(t, nodepb.CloseClientResponse{})
	defer stop()

	client := NewRaftGRPCClient(1, staticClusterState{nodes: []*cluster.NodeMeta{
		{Cluster: config.Cluster{LocalNodeID: 2, GRPC: config.GRPC{Endpoint: address}}},
	}}, config.TLS{Enabled: false}, true)
	client.rpcTimeout = 20 * time.Millisecond

	invokeBefore := histogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "rpc_invoke",
		"result": "timeout",
	})
	failureBefore := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("rpc_invoke", "deadline_exceeded"))

	err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a")
	if err == nil {
		t.Fatal("expected RequestCloseClient to fail")
	}

	assertHistogramSampleCount(t, metric.CloseClientClientStageDurationSeconds, map[string]string{
		"stage":  "rpc_invoke",
		"result": "timeout",
	}, invokeBefore+1)
	if got := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("rpc_invoke", "deadline_exceeded")); got != failureBefore+1 {
		t.Fatalf("rpc invoke timeout failure counter = %v, want %v", got, failureBefore+1)
	}
}

func TestRequestCloseClientRecordsResponseValidationFailures(t *testing.T) {
	tests := []struct {
		name       string
		resp       nodepb.CloseClientResponse
		wantReason string
	}{
		{name: "not_found", resp: nodepb.CloseClientResponse{Success: false, Message: "client not found"}, wantReason: "not_found"},
		{name: "owner_conflict", resp: nodepb.CloseClientResponse{Success: false, Message: "owner token mismatch"}, wantReason: "owner_conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, stop := startCloseClientTestServer(t, tt.resp)
			defer stop()

			client := NewRaftGRPCClient(1, staticClusterState{nodes: []*cluster.NodeMeta{
				{Cluster: config.Cluster{LocalNodeID: 2, GRPC: config.GRPC{Endpoint: address}}},
			}}, config.TLS{Enabled: false}, true)

			failureBefore := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("response_validate", tt.wantReason))

			err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a")
			if err == nil {
				t.Fatal("expected RequestCloseClient to fail")
			}

			if got := testutil.ToFloat64(metric.CloseClientClientFailuresTotal.WithLabelValues("response_validate", tt.wantReason)); got != failureBefore+1 {
				t.Fatalf("response validation counter = %v, want %v", got, failureBefore+1)
			}
		})
	}
}

func TestRequestCloseClientFastFailsWhenPeerIsNotReady(t *testing.T) {
	tracker := cluster.NewTrafficTracker()
	tracker.SetNodeState(2, cluster.TrafficStateJoining, "activation_pending")

	client := NewRaftGRPCClient(1, staticClusterState{nodes: []*cluster.NodeMeta{
		{Cluster: config.Cluster{LocalNodeID: 2, GRPC: config.GRPC{Endpoint: "127.0.0.1:65535"}}},
	}}, config.TLS{Enabled: false}, true)
	client.SetTrafficStateStore(tracker)

	fastFailBefore := testutil.ToFloat64(metric.PeerRPCFastFailTotal.WithLabelValues("2", "node_not_ready"))

	err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("RequestCloseClient error code = %v, want %v (err=%v)", status.Code(err), codes.Unavailable, err)
	}

	if got := testutil.ToFloat64(metric.PeerRPCFastFailTotal.WithLabelValues("2", "node_not_ready")); got != fastFailBefore+1 {
		t.Fatalf("fast fail counter = %v, want %v", got, fastFailBefore+1)
	}
}

func TestRequestCloseClientEvictsShutdownConnectionAndRedials(t *testing.T) {
	address, stop := startCloseClientReadyTestServer(t, nodepb.CloseClientResponse{Success: true})
	defer stop()

	client := NewRaftGRPCClient(1, staticClusterState{nodes: []*cluster.NodeMeta{
		{Cluster: config.Cluster{LocalNodeID: 2, GRPC: config.GRPC{Endpoint: address}}},
	}}, config.TLS{Enabled: false}, true)

	if err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a"); err != nil {
		t.Fatalf("first RequestCloseClient returned error: %v", err)
	}

	client.mu.RLock()
	cached := client.conns[2]
	client.mu.RUnlock()
	if cached == nil {
		t.Fatal("expected cached connection after first request")
	}
	_ = cached.Close()

	evictionsBefore := testutil.ToFloat64(metric.PeerConnEvictionsTotal.WithLabelValues("2", "shutdown"))

	if err := client.RequestCloseClient(context.Background(), 2, "client-a", "owner-a"); err != nil {
		t.Fatalf("second RequestCloseClient returned error: %v", err)
	}

	if got := testutil.ToFloat64(metric.PeerConnEvictionsTotal.WithLabelValues("2", "shutdown")); got != evictionsBefore+1 {
		t.Fatalf("shutdown eviction counter = %v, want %v", got, evictionsBefore+1)
	}
}

type closeClientTestServer struct {
	nodepb.UnimplementedClientCenterServer
	response nodepb.CloseClientResponse
	delay    time.Duration
}

func (s *closeClientTestServer) CloseClient(ctx context.Context, _ *nodepb.CloseClientRequest) (*nodepb.CloseClientResponse, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	resp := s.response
	return &resp, nil
}

func startCloseClientTestServer(t *testing.T, response nodepb.CloseClientResponse) (string, func()) {
	return startCloseClientReadyTestServerWithDelay(t, response, 100*time.Millisecond)
}

func startCloseClientReadyTestServer(t *testing.T, response nodepb.CloseClientResponse) (string, func()) {
	return startCloseClientReadyTestServerWithDelay(t, response, 0)
}

func startCloseClientReadyTestServerWithDelay(t *testing.T, response nodepb.CloseClientResponse, delay time.Duration) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	nodepb.RegisterClientCenterServer(server, &closeClientTestServer{
		response: response,
		delay:    delay,
	})
	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
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
