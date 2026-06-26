package client

import (
	"context"
	"net"
	"testing"
	"time"

	sessionmemory "github.com/BAN1ce/skyTree/internal/broker/sessioncenter/memory"
	submemory "github.com/BAN1ce/skyTree/internal/broker/subcenter/memory"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func TestStoreQoS2PublishPersistsIncomingWaitingPubrel(t *testing.T) {
	center := sessionmemory.NewCore()
	c := NewClient(&bufferConn{}, WithSessionCenter(center))
	c.ID = "incoming-qos2-persist"
	c.sessionExpiryInterval = 60
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	handler := NewClientHandler(c)

	if _, err := handler.storeQoS2Publish(context.Background(), &packets.Publish{
		Topic:      "qos2/persist",
		QoS:        2,
		PacketID:   77,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}, packets.PubrecSuccess); err != nil {
		t.Fatalf("store qos2 publish: %v", err)
	}

	resp, err := center.GetSession(context.Background(), &proto_session.ReadSessionRequest{ClientID: c.ID})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	msgs := resp.GetSession().GetUnfinishedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected one incoming unfinished message, got %+v", msgs)
	}
	got := msgs[0]
	if got.GetIsOutgoing() || got.GetPacketID() != 77 || got.GetQos() != 2 || got.GetState() != proto_session.UnfinishedMessage_WAITING_PUBREL {
		t.Fatalf("unexpected incoming unfinished message: %+v", got)
	}
	if len(got.GetPublishPacket()) == 0 {
		t.Fatal("expected encoded publish packet to be persisted")
	}
}

func TestHandlePubRelRemovesPersistedIncomingQoS2(t *testing.T) {
	center := sessionmemory.NewCore()
	subCenter := submemory.NewMemorySubCenter()
	serverConn, peerConn := net.Pipe()
	defer serverConn.Close()
	defer peerConn.Close()

	c := NewClient(
		serverConn,
		WithSessionCenter(center),
		WithSubCenter(subCenter),
		WithStateRouter(newTestInProcessStateRouter(t, center, subCenter)),
	)
	c.ID = "incoming-qos2-remove"
	c.setOwnerToken("owner-token")
	c.sessionExpiryInterval = 60
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	handler := NewClientHandler(c)

	if _, err := handler.storeQoS2Publish(context.Background(), &packets.Publish{
		Topic:      "qos2/remove",
		QoS:        2,
		PacketID:   78,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}, packets.PubrecSuccess); err != nil {
		t.Fatalf("store qos2 publish: %v", err)
	}

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = peerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(peerConn, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	if err := handler.handlePubRel(context.Background(), &packets.Pubrel{
		PacketID:   78,
		ReasonCode: packets.PubrelSuccess,
	}); err != nil {
		t.Fatalf("handle pubrel: %v", err)
	}

	select {
	case cp := <-readCh:
		pubcomp, ok := cp.Content.(*packets.Pubcomp)
		if !ok {
			t.Fatalf("expected PUBCOMP, got %T", cp.Content)
		}
		if pubcomp.PacketID != 78 || pubcomp.ReasonCode != packets.PubcompSuccess {
			t.Fatalf("unexpected PUBCOMP: %+v", pubcomp)
		}
	case err := <-errCh:
		t.Fatalf("read pubcomp: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBCOMP")
	}
	if _, ok := c.QoS2.Read(78); ok {
		t.Fatal("expected in-memory incoming qos2 state to be removed")
	}
	resp, err := center.GetSession(context.Background(), &proto_session.ReadSessionRequest{ClientID: c.ID})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if got := resp.GetSession().GetUnfinishedMessages(); len(got) != 0 {
		t.Fatalf("expected persisted incoming qos2 state to be removed, got %+v", got)
	}
}
