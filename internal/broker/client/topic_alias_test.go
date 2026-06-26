package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/clientalive"
	"github.com/BAN1ce/skyTree/logger"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

func TestHandlePublish_InvalidAlias_SendsDisconnectAndCloses(t *testing.T) {
	// Ensure global config exists; test relies on default env-defaults.
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1,
		WithKeepAliveTracker(clientalive.NewTracker()),
		WithSessionCenter(&fakeSessionCenter{}),
		WithConfig(Config{
			BrokerConfig: config.Broker{
				ConnectAckProperty: config.ConnectAckProperty{
					TopicAliasMaximum: 10,
				},
			},
		}),
	)
	c.ID = "client-1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	h := NewClientHandler(c)

	alias := uint16(1)
	p := &packets.Publish{
		Topic:      "",
		PacketID:   1,
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}

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

	_ = h.handlePublish(context.Background(), p)

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected reason code 0x82, got 0x%x", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for DISCONNECT")
	}
}
