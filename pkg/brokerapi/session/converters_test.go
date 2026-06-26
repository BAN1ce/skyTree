package session

import (
	"testing"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestWillMessageToProtoPreservesWillDelayInterval(t *testing.T) {
	willDelay := uint32(30)

	got := WillMessageToProto(&packets.Connect{
		WillFlag:    true,
		WillTopic:   "will/topic",
		WillMessage: []byte("offline"),
		WillProperties: &packets.PublishProperties{
			WillDelayInterval: &willDelay,
		},
	})
	if got == nil {
		t.Fatal("expected will message")
	}
	if got.WillDelayInterval == nil || got.GetWillDelayInterval() != willDelay {
		t.Fatalf("expected will delay interval %d, got %+v", willDelay, got.WillDelayInterval)
	}
}

func TestConvertIncomingUnfinishedMessagePreservesPublishPacket(t *testing.T) {
	msg := &brokerpublish.Message{
		AckReasonCode: packets.PubrecNoMatchingSubscribers,
		Publish: &packets.Publish{
			Topic:    "qos2/incoming",
			PacketID: 9,
			QoS:      2,
			Payload:  []byte("payload"),
		},
	}

	got := ConvertToProtoUnfinishedMessage(msg, false)
	if got == nil {
		t.Fatal("expected unfinished message")
	}
	if len(got.GetPublishPacket()) == 0 {
		t.Fatal("expected incoming QoS2 unfinished message to carry encoded publish packet")
	}
	if got.GetAckReasonCode() != uint32(packets.PubrecNoMatchingSubscribers) {
		t.Fatalf("expected ack reason code 0x%X, got 0x%X", packets.PubrecNoMatchingSubscribers, got.GetAckReasonCode())
	}
}
