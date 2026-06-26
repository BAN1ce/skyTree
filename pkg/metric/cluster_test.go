package metric

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordClusterHealthMetrics(t *testing.T) {
	RecordClusterHealth("key_store", 2, 1, time.Millisecond)
}

func TestRecordRaftRequestRecordsTimeout(t *testing.T) {
	before := testutil.ToFloat64(RaftTimeoutsTotal.WithLabelValues("key_store", "2", "write"))

	RecordRaftRequest("key_store", 2, "write", context.DeadlineExceeded, time.Millisecond, 128)

	after := testutil.ToFloat64(RaftTimeoutsTotal.WithLabelValues("key_store", "2", "write"))
	if after != before+1 {
		t.Fatalf("raft timeout counter = %v, want %v", after, before+1)
	}
}

func TestSetRaftGroupStarted(t *testing.T) {
	SetRaftGroupStarted("session_center", 3, true)

	if got := testutil.ToFloat64(RaftGroupStarted.WithLabelValues("session_center", "3")); got != 1 {
		t.Fatalf("raft group started = %v, want 1", got)
	}
}
