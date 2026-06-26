package client

import (
	"context"
	"testing"
	"time"

	sessionmemory "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/memory"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

func TestRemoveOutgoingUnfinishedFromSessionPreservesPersistentSession(t *testing.T) {
	center := sessionmemory.NewCore()
	clientID := "client-a"
	sessionExpiry := uint32(60)
	removeID := uuid.New()
	keepID := uuid.New()

	if err := center.SaveOfflineState(context.Background(), &proto_session.SaveOfflineStateRequest{
		ClientID:              clientID,
		SessionExpiryInterval: sessionExpiry,
		NowUnixNano:           time.Now().UnixNano(),
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:  removeID.String(),
				PacketID:   10,
				Qos:        2,
				State:      proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing: true,
			},
			{
				MessageID:  keepID.String(),
				PacketID:   11,
				Qos:        2,
				State:      proto_session.UnfinishedMessage_WAITING_PUBREL,
				IsOutgoing: false,
			},
		},
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	c := NewClient(&bufferConn{}, WithSessionCenter(center))
	c.ID = clientID
	c.removeOutgoingUnfinishedFromSession(removeID)

	resp, err := center.GetSession(context.Background(), &proto_session.ReadSessionRequest{
		ClientID: clientID,
	})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if resp == nil || !resp.GetExist() || resp.GetSession() == nil {
		t.Fatal("expected persistent session to remain after outgoing cleanup")
	}
	if got := resp.GetSession().GetSessionExpiryInterval(); got != sessionExpiry {
		t.Fatalf("expected session expiry %d, got %d", sessionExpiry, got)
	}
	msgs := resp.GetSession().GetUnfinishedMessages()
	if len(msgs) != 1 || msgs[0].GetMessageID() != keepID.String() {
		t.Fatalf("expected only non-target unfinished message to remain, got %+v", msgs)
	}
}
