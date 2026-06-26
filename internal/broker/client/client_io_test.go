package client

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestWriteReturnsWhenPeerDoesNotRead(t *testing.T) {
	serverConn, peerConn := net.Pipe()
	defer serverConn.Close()
	defer peerConn.Close()

	c := NewClient(serverConn, WithConfig(Config{WriteTimeout: 20 * time.Millisecond}))
	c.ID = "blocked-writer"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	packet := packets.NewControlPacket(packets.PINGRESP)
	packet.Content = &packets.Pingresp{}

	start := time.Now()
	err := c.Write(&clientcap.WritePacket{Packet: packet})
	if err == nil {
		t.Fatal("expected write timeout error")
	}
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("write did not respect timeout, elapsed=%s err=%v", elapsed, err)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected net timeout error, got %T: %v", err, err)
	}
}
