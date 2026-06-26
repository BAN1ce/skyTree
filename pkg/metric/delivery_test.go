package metric

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNormalizeDeliveryMetricLabels(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		qos         int
		result      string
		mode        string
		attempt     string
		wantPath    string
		wantQoS     string
		wantResult  string
		wantMode    string
		wantAttempt string
	}{
		{
			name:        "known labels pass through",
			path:        "shared",
			qos:         2,
			result:      "duplicate",
			mode:        "event_remote",
			attempt:     "retransmit",
			wantPath:    "shared",
			wantQoS:     "2",
			wantResult:  "duplicate",
			wantMode:    "event_remote",
			wantAttempt: "retransmit",
		},
		{
			name:        "unknown labels fall back to bounded values",
			path:        "tenant-a",
			qos:         9,
			result:      "client-a",
			mode:        "node-1",
			attempt:     "retry-7",
			wantPath:    "normal",
			wantQoS:     "0",
			wantResult:  "error",
			wantMode:    "direct",
			wantAttempt: "initial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDeliveryPath(tt.path); got != tt.wantPath {
				t.Fatalf("normalizeDeliveryPath(%q) = %q, want %q", tt.path, got, tt.wantPath)
			}
			if got := normalizeDeliveryQoS(tt.qos); got != tt.wantQoS {
				t.Fatalf("normalizeDeliveryQoS(%d) = %q, want %q", tt.qos, got, tt.wantQoS)
			}
			if got := normalizeDeliveryResult(tt.result); got != tt.wantResult {
				t.Fatalf("normalizeDeliveryResult(%q) = %q, want %q", tt.result, got, tt.wantResult)
			}
			if got := normalizeDeliveryWakeMode(tt.mode); got != tt.wantMode {
				t.Fatalf("normalizeDeliveryWakeMode(%q) = %q, want %q", tt.mode, got, tt.wantMode)
			}
			if got := normalizeDeliveryAttempt(tt.attempt); got != tt.wantAttempt {
				t.Fatalf("normalizeDeliveryAttempt(%q) = %q, want %q", tt.attempt, got, tt.wantAttempt)
			}
		})
	}
}

func TestRecordDeliverySendAttemptNormalizesLabels(t *testing.T) {
	before := testutil.ToFloat64(DeliverySendAttemptTotal.WithLabelValues("normal", "0", "initial"))

	RecordDeliverySendAttempt("client-a", 9, "retry-7")

	after := testutil.ToFloat64(DeliverySendAttemptTotal.WithLabelValues("normal", "0", "initial"))
	if after != before+1 {
		t.Fatalf("normalized send attempt counter = %v, want %v", after, before+1)
	}
}
