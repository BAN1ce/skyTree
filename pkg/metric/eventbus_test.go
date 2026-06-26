package metric

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNormalizeEventBusMetricLabels(t *testing.T) {
	if got := normalizeEventBusEnqueueResult("queued_after_drop"); got != "queued_after_drop" {
		t.Fatalf("normalizeEventBusEnqueueResult queued_after_drop = %q", got)
	}
	if got := normalizeEventBusEnqueueResult("tenant-a"); got != "queued" {
		t.Fatalf("normalizeEventBusEnqueueResult fallback = %q", got)
	}
	if got := normalizeEventBusDropReason("queue_overflow_oldest"); got != "queue_overflow_oldest" {
		t.Fatalf("normalizeEventBusDropReason queue_overflow_oldest = %q", got)
	}
	if got := normalizeEventBusDropReason("client-a"); got != "listener_stopped" {
		t.Fatalf("normalizeEventBusDropReason fallback = %q", got)
	}
}

func TestRecordEventBusMetricsNormalizeLabels(t *testing.T) {
	enqueueBefore := testutil.ToFloat64(EventBusEnqueueTotal.WithLabelValues("queued"))
	dropBefore := testutil.ToFloat64(EventBusDropTotal.WithLabelValues("listener_stopped"))

	RecordEventBusEnqueue("unknown")
	RecordEventBusDrop("unknown")

	enqueueAfter := testutil.ToFloat64(EventBusEnqueueTotal.WithLabelValues("queued"))
	dropAfter := testutil.ToFloat64(EventBusDropTotal.WithLabelValues("listener_stopped"))

	if enqueueAfter != enqueueBefore+1 {
		t.Fatalf("enqueue counter = %v, want %v", enqueueAfter, enqueueBefore+1)
	}
	if dropAfter != dropBefore+1 {
		t.Fatalf("drop counter = %v, want %v", dropAfter, dropBefore+1)
	}
}

func TestSetEventBusListenerQueueDepthClampsNegative(t *testing.T) {
	SetEventBusListenerQueueDepth(-1)
	if got := testutil.ToFloat64(EventBusListenerQueueDepth); got != 0 {
		t.Fatalf("queue depth = %v, want 0", got)
	}
}
