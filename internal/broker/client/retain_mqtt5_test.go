package client

import (
	"testing"
	"time"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestRetainMessageRoundTripPreservesMQTT5PublishProperties(t *testing.T) {
	now := time.Unix(100, 0)
	expiry := uint32(60)
	payloadFormat := byte(1)
	publish := &packets.Publish{
		Topic:    "sensors/temp",
		QoS:      1,
		PacketID: 7,
		Retain:   true,
		Payload:  []byte("21.5"),
		Properties: &packets.PublishProperties{
			PayloadFormat: &payloadFormat,
			MessageExpiry: &expiry,
			ContentType:   "text/plain",
			User:          []packets.User{{Key: "source", Value: "test"}},
		},
	}

	retained, err := newRetainMessageFromPublish(publish, now, "publisher-a")
	if err != nil {
		t.Fatalf("create retain message: %v", err)
	}
	if retained.GetPublisherClientID() != "publisher-a" {
		t.Fatalf("publisher client id lost: %q", retained.GetPublisherClientID())
	}
	if got, want := retained.GetExpiredAtUnixNano(), now.Add(time.Duration(expiry)*time.Second).UnixNano(); got != want {
		t.Fatalf("expected explicit retained expiry %d, got %d", want, got)
	}

	got, ok := publishFromRetainMessage(retained, now.Add(10*time.Second))
	if !ok {
		t.Fatalf("expected retained publish to be available")
	}
	if got.QoS != 1 {
		t.Fatalf("expected retained qos=1, got %d", got.QoS)
	}
	if !got.Retain {
		t.Fatalf("expected retain flag to be set")
	}
	if got.Properties == nil || got.Properties.MessageExpiry == nil || *got.Properties.MessageExpiry != 50 {
		t.Fatalf("expected remaining message expiry 50, got %+v", got.Properties)
	}
	if got.Properties.ContentType != "text/plain" {
		t.Fatalf("content type lost: %+v", got.Properties)
	}
	if len(got.Properties.User) != 1 || got.Properties.User[0].Key != "source" || got.Properties.User[0].Value != "test" {
		t.Fatalf("user properties lost: %+v", got.Properties.User)
	}
}

func TestRetainMessageExpiredIsNotDelivered(t *testing.T) {
	now := time.Unix(100, 0)
	expiry := uint32(1)
	publish := &packets.Publish{
		Topic:      "sensors/temp",
		QoS:        1,
		PacketID:   7,
		Retain:     true,
		Payload:    []byte("21.5"),
		Properties: &packets.PublishProperties{MessageExpiry: &expiry},
	}

	retained, err := newRetainMessageFromPublish(publish, now, "publisher-a")
	if err != nil {
		t.Fatalf("create retain message: %v", err)
	}

	if got, ok := publishFromRetainMessage(retained, now.Add(2*time.Second)); ok {
		t.Fatalf("expected retained publish to expire, got %+v", got)
	}
}

func TestRetainedTopicFilterForSharedSubscriptionUsesActualFilter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "normal exact", in: "sensor/temp", want: "sensor/temp"},
		{name: "shared exact", in: "$share/group/sensor/temp", want: "sensor/temp"},
		{name: "shared wildcard", in: "$share/group/sensor/#", want: "sensor/#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retainedTopicFilterForSubscription(tt.in)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
