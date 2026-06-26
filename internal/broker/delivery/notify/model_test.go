package notify

import (
	"strings"
	"testing"

	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/proto/proto_event"
	"github.com/google/uuid"
	proto2 "google.golang.org/protobuf/proto"
)

func TestBuildPreparedNotifyWakeKeyRule(t *testing.T) {
	notifyPayload, emitKey, err := BuildPreparedNotify(deliveryevent.KindWake, "topic/a", nil)
	if err != nil {
		t.Fatalf("BuildPreparedNotify error: %v", err)
	}
	if notifyPayload == nil || notifyPayload.Kind != deliveryevent.KindWake {
		t.Fatalf("expected wake notify, got %+v", notifyPayload)
	}
	if !strings.HasPrefix(emitKey, "topic/a-") {
		t.Fatalf("expected wake key with topic prefix, got %s", emitKey)
	}
}

func TestBuildPreparedNotifyQoS0Direct(t *testing.T) {
	payload, reqID := encodeQoS0PayloadForTest(t)
	notifyPayload, emitKey, err := BuildPreparedNotify(deliveryevent.KindQoS0Direct, "topic/a", payload)
	if err != nil {
		t.Fatalf("BuildPreparedNotify error: %v", err)
	}
	if emitKey != reqID {
		t.Fatalf("expected req id as emit key, got %s", emitKey)
	}
	if notifyPayload == nil || notifyPayload.Message == nil {
		t.Fatalf("expected decoded message in qos0 notify")
	}
}

func TestBuildPreparedNotifyQoS0DirectMissingPayload(t *testing.T) {
	_, _, err := BuildPreparedNotify(deliveryevent.KindQoS0Direct, "topic/a", nil)
	if err == nil {
		t.Fatalf("expected error when qos0 payload is missing")
	}
}

func TestBuildClientNotifyPayloadAppliesClientOptions(t *testing.T) {
	base := &deliveryevent.Notify{
		Kind:         deliveryevent.KindQoS0Direct,
		PublishTopic: "topic/a",
		Message:      &brokerpublish.Message{},
	}
	payload := BuildClientNotifyPayload(base, "c1", map[string]ClientDeliveryOptions{
		"c1": {
			NoLocal:             true,
			RAP:                 false,
			SubscriptionIDsJSON: "[1,2]",
		},
	})
	if payload == nil || !payload.NoLocal || payload.RAP || payload.SubscriptionIDsJSON != "[1,2]" {
		t.Fatalf("unexpected payload options: %+v", payload)
	}
}

func TestSharedWakePayloadRoundTrip(t *testing.T) {
	payload, err := EncodeSharedWakePayload(SharedWakePayload{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		TaskID:      "task-1",
	})
	if err != nil {
		t.Fatalf("EncodeSharedWakePayload: %v", err)
	}
	got, err := DecodeSharedWakePayload(payload)
	if err != nil {
		t.Fatalf("DecodeSharedWakePayload: %v", err)
	}
	if got.ShareGroup != "g1" || got.TopicFilter != "a/b" || got.TaskID != "task-1" {
		t.Fatalf("unexpected shared wake payload: %+v", got)
	}
}

func TestSharedWakePayloadValidation(t *testing.T) {
	tests := []SharedWakePayload{
		{TopicFilter: "a/b", TaskID: "task-1"},
		{ShareGroup: "g1", TaskID: "task-1"},
		{ShareGroup: "g1", TopicFilter: "a/b"},
	}
	for _, tt := range tests {
		if _, err := EncodeSharedWakePayload(tt); err == nil {
			t.Fatalf("expected validation error for %+v", tt)
		}
	}
	if _, err := DecodeSharedWakePayload(nil); err == nil {
		t.Fatal("expected decode error for empty payload")
	}
}

func encodeQoS0PayloadForTest(t *testing.T) ([]byte, string) {
	t.Helper()
	reqID := uuid.NewString()
	raw, err := serializer.Serializer.Encode(&brokerpublish.Message{})
	if err != nil {
		t.Fatalf("encode broker message: %v", err)
	}
	req := &proto_event.Request{
		ID:   reqID,
		Type: proto_event.RequestType_EMIT_EVENT,
		Data: raw,
	}
	payload, err := proto2.Marshal(req)
	if err != nil {
		t.Fatalf("marshal proto_event request: %v", err)
	}
	return payload, reqID
}
