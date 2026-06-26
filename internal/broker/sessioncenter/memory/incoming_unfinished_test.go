package memory

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func TestUpsertAndRemoveIncomingUnfinishedPreservesSessionState(t *testing.T) {
	logger.LoadForTest()
	core := NewCore()
	ctx := context.Background()
	clientID := "client-a"
	ownerToken := "owner-a"
	now := time.Now().UnixNano()
	cursor := &proto_session.OutgoingReplayCursor{
		TaskUnixMicro: time.Unix(99, 0).UnixMicro(),
		TaskID:        "00000000-0000-0000-0000-000000000701",
		Generation:    7,
	}
	will := &proto_session.WillMessage{
		Topic:   "will/topic",
		Payload: []byte("will"),
		Qos:     1,
	}
	outgoing := &proto_session.UnfinishedMessage{
		MessageID:  "00000000-0000-0000-0000-000000000702",
		PacketID:   10,
		Qos:        2,
		State:      proto_session.UnfinishedMessage_WAITING_PUBCOMP,
		IsOutgoing: true,
	}

	if _, err := core.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              clientID,
		WillMessage:           will,
		SessionExpiryInterval: 60,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("open session: %v", err)
	}
	if _, err := core.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{
		Owner: &proto_session.SessionOwner{
			ClientID:   clientID,
			NodeID:     1,
			OwnerToken: ownerToken,
			Online:     true,
		},
	}); err != nil {
		t.Fatalf("take over owner: %v", err)
	}
	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              clientID,
		UnfinishedMessages:    []*proto_session.UnfinishedMessage{outgoing},
		WillMessage:           will,
		OutgoingReplayCursor:  cursor,
		SessionExpiryInterval: 60,
		NowUnixNano:           now,
		OwnerToken:            ownerToken,
	}); err != nil {
		t.Fatalf("seed offline state: %v", err)
	}

	incoming := &proto_session.UnfinishedMessage{
		MessageID:     "00000000-0000-0000-0000-000000000703",
		PacketID:      20,
		Qos:           2,
		State:         proto_session.UnfinishedMessage_WAITING_PUBREL,
		IsOutgoing:    false,
		PublishPacket: []byte{0x34, 0x02, 0x00, 0x14},
		AckReasonCode: 0,
		FirstPubTime:  time.Unix(100, 1).UnixMicro(),
	}
	if err := core.UpsertIncomingUnfinished(ctx, &proto_session.UpsertIncomingUnfinishedRequest{
		ClientID:          clientID,
		PacketID:          incoming.GetPacketID(),
		UnfinishedMessage: incoming,
		OwnerToken:        ownerToken,
		NowUnixNano:       now,
	}); err != nil {
		t.Fatalf("upsert incoming: %v", err)
	}

	resp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: clientID})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if resp.GetSession().GetWillMessage().GetTopic() != will.GetTopic() {
		t.Fatalf("will was not preserved: %+v", resp.GetSession().GetWillMessage())
	}
	if got := resp.GetSession().GetOutgoingReplayCursor(); got.GetTaskID() != cursor.GetTaskID() {
		t.Fatalf("cursor was not preserved: %+v", got)
	}
	if got := resp.GetSession().GetUnfinishedMessages(); len(got) != 2 {
		t.Fatalf("expected outgoing and incoming unfinished messages, got %+v", got)
	}

	if err := core.RemoveIncomingUnfinished(ctx, &proto_session.RemoveIncomingUnfinishedRequest{
		ClientID:    clientID,
		PacketID:    incoming.GetPacketID(),
		OwnerToken:  ownerToken,
		NowUnixNano: now,
	}); err != nil {
		t.Fatalf("remove incoming: %v", err)
	}
	resp, err = core.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: clientID})
	if err != nil {
		t.Fatalf("read after remove: %v", err)
	}
	msgs := resp.GetSession().GetUnfinishedMessages()
	if len(msgs) != 1 || !msgs[0].GetIsOutgoing() || msgs[0].GetMessageID() != outgoing.GetMessageID() {
		t.Fatalf("expected only outgoing unfinished to remain, got %+v", msgs)
	}
}
