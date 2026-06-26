package serializer

import (
	"testing"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestEncodeSetsExpiryTimesFromPublishMessageExpiry(t *testing.T) {
	expiry := uint32(2)
	msg := &brokerpublish.Message{
		Publish: &packets.Publish{
			Topic:      "a/b",
			Payload:    []byte("payload"),
			Properties: &packets.PublishProperties{MessageExpiry: &expiry},
		},
	}

	raw, err := Serializer.Encode(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := Serializer.Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CreatedTime <= 0 {
		t.Fatalf("expected CreatedTime to be set")
	}
	if got.ExpiredTime <= got.CreatedTime {
		t.Fatalf("expected ExpiredTime > CreatedTime, created=%d expired=%d", got.CreatedTime, got.ExpiredTime)
	}
}
