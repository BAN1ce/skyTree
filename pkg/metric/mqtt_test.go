package metric

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordMQTTPacketNormalizesUnknownLabels(t *testing.T) {
	before := testutil.ToFloat64(MQTTPacketsTotal.WithLabelValues("unknown", "unknown"))

	RecordMQTTPacket("sideways", 99)

	after := testutil.ToFloat64(MQTTPacketsTotal.WithLabelValues("unknown", "unknown"))
	if after != before+1 {
		t.Fatalf("normalized MQTT packet counter = %v, want %v", after, before+1)
	}
}

func TestRecordMQTTPublishPacketNormalizesQoS(t *testing.T) {
	before := testutil.ToFloat64(MQTTPublishPacketsTotal.WithLabelValues("0", "in"))

	RecordMQTTPublishPacket(9, "in")

	after := testutil.ToFloat64(MQTTPublishPacketsTotal.WithLabelValues("0", "in"))
	if after != before+1 {
		t.Fatalf("normalized MQTT publish counter = %v, want %v", after, before+1)
	}
}

func TestRecordPublishRetryCreateFailedHasBoundedLabels(t *testing.T) {
	before := testutil.ToFloat64(PublishRetryCreateFailed.WithLabelValues("0", "unknown"))

	RecordPublishRetryCreateFailed(9, "client-a")

	after := testutil.ToFloat64(PublishRetryCreateFailed.WithLabelValues("0", "unknown"))
	if after != before+1 {
		t.Fatalf("retry create failure counter = %v, want %v", after, before+1)
	}
}
