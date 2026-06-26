package client

import (
	"bytes"
	"context"
	"net"
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func TestRestoreIncomingQoS2FromSession(t *testing.T) {
	publish := &packets.Publish{
		Topic:    "qos2/incoming",
		PacketID: 42,
		QoS:      2,
		Payload:  []byte("payload"),
	}
	var buf bytes.Buffer
	if _, err := wire.Write(&buf, publish.ToControlPacket(), wire.EncodeOptions{}); err != nil {
		t.Fatalf("write publish: %v", err)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	cl := NewClient(c1)
	cl.ID = "restore-incoming"
	cl.restoreIncomingQoS2FromSession(context.Background(), &proto_session.Session{
		ClientID: "restore-incoming",
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				PacketID:      42,
				Qos:           2,
				State:         proto_session.UnfinishedMessage_WAITING_PUBREL,
				IsOutgoing:    false,
				PublishPacket: buf.Bytes(),
				AckReasonCode: uint32(packets.PubrecNoMatchingSubscribers),
			},
		},
	})

	msg, ok := cl.QoS2.Read(42)
	if !ok {
		t.Fatal("expected incoming QoS2 state to be restored")
	}
	got := msg.GetPublish()
	if got == nil || got.Topic != "qos2/incoming" || string(got.Payload) != "payload" {
		t.Fatalf("unexpected restored publish: %+v", got)
	}
	if msg.AckReasonCode != packets.PubrecNoMatchingSubscribers {
		t.Fatalf("expected ack reason code 0x%X, got 0x%X", packets.PubrecNoMatchingSubscribers, msg.AckReasonCode)
	}
}
