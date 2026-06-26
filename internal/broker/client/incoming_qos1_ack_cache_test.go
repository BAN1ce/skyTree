package client

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/google/uuid"
)

func TestHandlePublish_QoS1_DupRetransmitAfterPubAckProcessesAsNewMessage(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	var processed atomic.Int64
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(ctx context.Context, msg *brokerpublish.Message) error {
		processed.Add(1)
		return nil
	})))
	c.ID = "client-1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	h := NewClientHandler(c)

	runPublishAndReadPubAck := func(publish *packets.Publish) *packets.Puback {
		t.Helper()
		readCh := make(chan *packets.ControlPacket, 1)
		errCh := make(chan error, 1)
		go func() {
			_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
			cp, err := wire.Decode(c2, wire.DecodeOptions{})
			if err != nil {
				errCh <- err
				return
			}
			readCh <- cp
		}()

		if err := h.handlePublish(context.Background(), publish); err != nil {
			t.Fatalf("handle publish: %v", err)
		}

		var cp *packets.ControlPacket
		select {
		case cp = <-readCh:
		case err := <-errCh:
			t.Fatalf("read packet error: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for PUBACK")
		}

		if cp.Type != packets.PUBACK {
			t.Fatalf("expected PUBACK, got %s", cp.PacketType())
		}
		ack, ok := cp.Content.(*packets.Puback)
		if !ok {
			t.Fatalf("expected Puback, got %T", cp.Content)
		}
		return ack
	}

	firstAck := runPublishAndReadPubAck(&packets.Publish{
		Topic:    "a/b",
		PacketID: 7,
		QoS:      1,
		Payload:  []byte("first"),
	})
	if firstAck.PacketID != 7 {
		t.Fatalf("expected first PacketID=7, got %d", firstAck.PacketID)
	}

	secondAck := runPublishAndReadPubAck(&packets.Publish{
		Topic:     "a/b",
		PacketID:  7,
		QoS:       1,
		Duplicate: true,
		Payload:   []byte("second"),
	})
	if secondAck.PacketID != 7 {
		t.Fatalf("expected second PacketID=7, got %d", secondAck.PacketID)
	}

	if processed.Load() != 2 {
		t.Fatalf("expected both publishes to be processed as application messages, got %d", processed.Load())
	}
}

func TestHandlePublish_QoS1_ReusedPacketIDAfterAckProcessesNewPublish(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	var processed atomic.Int64
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(ctx context.Context, msg *brokerpublish.Message) error {
		processed.Add(1)
		return nil
	})))
	c.ID = "client-1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	h := NewClientHandler(c)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	if err := h.handlePublish(context.Background(), &packets.Publish{
		Topic:    "a/new",
		PacketID: 7,
		QoS:      1,
		Payload:  []byte("new payload"),
	}); err != nil {
		t.Fatalf("handlePublish: %v", err)
	}

	if processed.Load() != 1 {
		t.Fatalf("expected new publish to be processed once, got %d", processed.Load())
	}

	select {
	case cp := <-readCh:
		ack, ok := cp.Content.(*packets.Puback)
		if !ok {
			t.Fatalf("expected PUBACK, got %T", cp.Content)
		}
		if ack.PacketID != 7 {
			t.Fatalf("expected PacketID=7, got %d", ack.PacketID)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for PUBACK")
	}
}

func TestHandlePublish_QoS1_RetryReusesMessageIDUntilPubAckWritten(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	logger.LoadForTest()

	var (
		callCount atomic.Int64
		firstID   uuid.UUID
		retryID   uuid.UUID
		newID     uuid.UUID
	)

	c := NewClient(&callbackConn{}, WithStateRouter(newTestPublishStateRouter(func(ctx context.Context, msg *brokerpublish.Message) error {
		switch callCount.Add(1) {
		case 1:
			firstID = msg.MessageID
			return errors.New("persist failed")
		case 2:
			retryID = msg.MessageID
		case 3:
			newID = msg.MessageID
		}
		return nil
	})))
	c.ID = "client-1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	h := NewClientHandler(c)

	// First QoS1 publish fails before PUBACK write: retry should reuse the same logical message ID.
	if err := h.handlePublish(context.Background(), &packets.Publish{
		Topic:    "a/b",
		PacketID: 7,
		QoS:      1,
		Payload:  []byte("payload"),
	}); err == nil {
		t.Fatal("expected first publish to fail")
	}
	if err := h.handlePublish(context.Background(), &packets.Publish{
		Topic:     "a/b",
		PacketID:  7,
		QoS:       1,
		Payload:   []byte("payload"),
		Duplicate: true,
	}); err != nil {
		t.Fatalf("retry publish: %v", err)
	}

	// After PUBACK is written, the same PacketID in a new publish cycle must use a new message ID.
	if err := h.handlePublish(context.Background(), &packets.Publish{
		Topic:    "a/b",
		PacketID: 7,
		QoS:      1,
		Payload:  []byte("payload-2"),
	}); err != nil {
		t.Fatalf("new publish after ack: %v", err)
	}

	if firstID == uuid.Nil || retryID == uuid.Nil || newID == uuid.Nil {
		t.Fatalf("expected non-nil message IDs, got first=%s retry=%s new=%s", firstID, retryID, newID)
	}
	if firstID != retryID {
		t.Fatalf("expected retry to reuse message ID, got first=%s retry=%s", firstID, retryID)
	}
	if newID == retryID {
		t.Fatalf("expected new publish cycle to use a new message ID, got %s", newID)
	}
}
