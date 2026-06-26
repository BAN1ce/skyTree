package client

import (
	"context"
	"io"
	"testing"
	"time"

	sessionmemory "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/memory"
	submemory "github.com/BAN1ce/skyTree/internal/broker/subcenter/memory"
	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestCollectOutgoingStateSplitsQoS1ReplayCursorFromUnfinished(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	qos1TaskID := uuid.MustParse("00000000-0000-0000-0000-000000000401")
	qos2MessageID := uuid.MustParse("00000000-0000-0000-0000-000000000402")
	qos2TaskID := uuid.MustParse("00000000-0000-0000-0000-000000000403")
	firstTS := time.UnixMicro(1000)

	cl := NewClient(&bufferConn{})
	cl.ID = "test-client"
	cl.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:     10,
		QoS:          1,
		TaskTS:       firstTS,
		TaskID:       qos1TaskID,
		Generation:   2,
		MessageID:    uuid.New(),
		FirstPubTime: firstTS,
		State:        outInflightWaitingPubAck,
	})
	cl.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:     11,
		QoS:          2,
		TaskTS:       firstTS.Add(time.Second),
		TaskID:       qos2TaskID,
		Generation:   2,
		MessageID:    qos2MessageID,
		FirstPubTime: firstTS.Add(time.Second),
		State:        outInflightWaitingPubRec,
	})

	unfinished := cl.collectOutgoingUnfinishedMessages()
	if len(unfinished) != 1 {
		t.Fatalf("expected only qos2 unfinished, got %+v", unfinished)
	}
	if unfinished[0].GetQos() != 2 || unfinished[0].GetState() != proto_session.UnfinishedMessage_WAITING_PUBREC {
		t.Fatalf("expected qos2 waiting pubrec unfinished, got %+v", unfinished[0])
	}

	cursor := cl.collectOutgoingReplayCursor()
	if cursor == nil {
		t.Fatal("expected qos1 replay cursor")
	}
	if cursor.GetTaskID() != qos1TaskID.String() {
		t.Fatalf("expected replay task id %s, got %s", qos1TaskID, cursor.GetTaskID())
	}
	if cursor.GetTaskUnixMicro() != firstTS.UnixMicro() {
		t.Fatalf("expected replay task time %d, got %d", firstTS.UnixMicro(), cursor.GetTaskUnixMicro())
	}
	if cursor.GetGeneration() != 2 {
		t.Fatalf("expected replay generation 2, got %d", cursor.GetGeneration())
	}
}

func TestPersistUnfinishedMessagesToSessionDoesNotCollectIncomingQoS2(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	center := sessionmemory.NewCore()
	ctx := context.Background()
	clientID := "test-client-close"
	persistedIncoming := &proto_session.UnfinishedMessage{
		MessageID:     uuid.NewString(),
		PacketID:      21,
		Qos:           2,
		State:         proto_session.UnfinishedMessage_WAITING_PUBREL,
		IsOutgoing:    false,
		PublishPacket: []byte{0x34, 0x02, 0x00, 0x15},
	}
	if err := center.UpsertIncomingUnfinished(ctx, &proto_session.UpsertIncomingUnfinishedRequest{
		ClientID:          clientID,
		PacketID:          persistedIncoming.GetPacketID(),
		UnfinishedMessage: persistedIncoming,
		NowUnixNano:       time.Now().UnixNano(),
	}); err != nil {
		t.Fatalf("seed incoming unfinished: %v", err)
	}

	subCenter := submemory.NewMemorySubCenter()
	cl := NewClient(
		&bufferConn{},
		WithSessionCenter(center),
		WithSubCenter(subCenter),
		WithStateRouter(newTestInProcessStateRouter(t, center, subCenter)),
	)
	cl.ID = clientID
	cl.setOwnerToken("owner-token")
	cl.sessionExpiryInterval = 60
	cl.QoS2.Store(&brokerpublish.Message{
		MessageID: uuid.New(),
		Publish: &packets.Publish{
			Topic:      "qos2/not-from-close",
			QoS:        2,
			PacketID:   22,
			Payload:    []byte("value"),
			Properties: &packets.PublishProperties{},
		},
	})
	outgoingMessageID := uuid.New()
	cl.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:     23,
		QoS:          2,
		TaskTS:       time.Now(),
		TaskID:       uuid.New(),
		MessageID:    outgoingMessageID,
		FirstPubTime: time.Now(),
		State:        outInflightWaitingPubRec,
	})

	cl.persistUnfinishedMessagesToSession(ctx)

	resp, err := center.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: clientID})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	msgs := resp.GetSession().GetUnfinishedMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected persisted incoming and outgoing unfinished, got %+v", msgs)
	}
	seenPersistedIncoming := false
	seenOutgoing := false
	for _, msg := range msgs {
		switch {
		case !msg.GetIsOutgoing() && msg.GetPacketID() == persistedIncoming.GetPacketID():
			seenPersistedIncoming = true
		case msg.GetIsOutgoing() && msg.GetMessageID() == outgoingMessageID.String():
			seenOutgoing = true
		case !msg.GetIsOutgoing() && msg.GetPacketID() == 22:
			t.Fatalf("close path should not persist in-memory incoming qos2 state: %+v", msg)
		}
	}
	if !seenPersistedIncoming || !seenOutgoing {
		t.Fatalf("expected persisted incoming=%v and outgoing=%v in %+v", seenPersistedIncoming, seenOutgoing, msgs)
	}
}
