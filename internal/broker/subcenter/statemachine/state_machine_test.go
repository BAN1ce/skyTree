package statemachine

import (
	"errors"
	"strings"
	"testing"

	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
	proto2 "google.golang.org/protobuf/proto"
)

func TestUpdateReturnsMarshalError(t *testing.T) {
	originalMarshal := protoMarshal
	protoMarshal = func(proto2.Message) ([]byte, error) {
		return nil, errors.New("marshal boom")
	}
	t.Cleanup(func() {
		protoMarshal = originalMarshal
	})

	sm := NewStateMachine()
	data, err := EncodeUpdate(&proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a/#", QoS: 1},
		},
	})
	if err != nil {
		t.Fatalf("encode update failed: %v", err)
	}

	_, err = sm.Update(data)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal sub response failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
