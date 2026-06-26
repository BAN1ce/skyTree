package metric

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecordGRPCClientRequestRecordsResultAndCode(t *testing.T) {
	before := testutil.ToFloat64(
		GRPCClientRequestsTotal.WithLabelValues("ClientCenter", "CloseClient", "2", "error", codes.Unavailable.String()),
	)

	err := status.Error(codes.Unavailable, "unavailable")
	RecordGRPCClientRequest("ClientCenter", "CloseClient", 2, status.Code(err), err, time.Millisecond, 16, 8)

	after := testutil.ToFloat64(
		GRPCClientRequestsTotal.WithLabelValues("ClientCenter", "CloseClient", "2", "error", codes.Unavailable.String()),
	)
	if after != before+1 {
		t.Fatalf("gRPC client request counter = %v, want %v", after, before+1)
	}
}

func TestGRPCClientInflightEndDecrementsGauge(t *testing.T) {
	done := BeginGRPCClientRequest("ClientCenter", "CloseClient", 3)
	done()

	if got := testutil.ToFloat64(GRPCClientInflight.WithLabelValues("ClientCenter", "CloseClient", "3")); got != 0 {
		t.Fatalf("gRPC client inflight = %v, want 0", got)
	}
}
