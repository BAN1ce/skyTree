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

func TestHandleConnectRegistersClientAliveTrackerAfterConnAck(t *testing.T) {
	cfg := mustLoadConfigForTest(t)
	cfg.Broker.ConnectAckProperty.ServerKeepAlive = -1
	logger.LoadForTest()

	tracker := clientalive.NewTracker()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	sessionCenter := &fakeSessionCenter{}
	subCenter := &recordingSubCenter{}
	c := NewClient(
		c1,
		WithConfig(clientConfigForTest(cfg.Broker)),
		WithSessionCenter(sessionCenter),
		WithSubCenter(subCenter),
		WithStateRouter(newTestInProcessStateRouter(t, sessionCenter, subCenter)),
		WithClientManager(NewManager()),
		WithKeepAliveTracker(tracker),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	readDone := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err := wire.Decode(c2, wire.DecodeOptions{})
		readDone <- err
	}()

	handler := NewClientHandler(c)
	if err := handler.handleConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "client-a",
		CleanStart:      true,
		KeepAlive:       2,
		Properties:      &packets.ConnectProperties{},
	}); err != nil {
		t.Fatalf("handleConnect: %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}

	lastAlive, ok := tracker.LastAlive("client-a")
	if !ok {
		t.Fatal("expected tracker to contain client-a after CONNACK")
	}
	if lastAlive.IsZero() {
		t.Fatal("expected tracker last alive to be set after CONNACK")
	}
	expireAt, ok := tracker.ExpireAt("client-a")
	if !ok {
		t.Fatal("expected tracker expire time to be set after CONNACK")
	}
	keepAlive := c.GetKeepAliveTime()
	if keepAlive <= 0 {
		t.Fatalf("GetKeepAliveTime() = %s, want positive negotiated keepalive", keepAlive)
	}
	wantExpireAt := lastAlive.Add(keepAlive + keepAlive/2)
	if !expireAt.Equal(wantExpireAt) {
		t.Fatalf("ExpireAt() = %s, want %s", expireAt, wantExpireAt)
	}
}

func TestHandlePacketRefreshesAliveTrackerForInboundPacketAfterConnect(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	tracker := clientalive.NewTracker()
	c := NewClient(
		&bufferConn{},
		WithKeepAliveTracker(tracker),
	)
	c.ID = "client-a"
	c.ownerToken = "owner-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)
	c.keepAlive = time.Second
	c.connAckAccepted.Store(true)

	oldAlive := time.Now().Add(-time.Minute)
	c.aliveTime.Store(oldAlive)
	tracker.Update("client-a", "owner-a", oldAlive, time.Second)

	err := NewClientHandler(c).HandlePacket(context.Background(), packets.NewControlPacket(packets.PINGREQ), c)
	if err != nil {
		t.Fatalf("HandlePacket(PINGREQ): %v", err)
	}

	lastAlive, ok := tracker.LastAlive("client-a")
	if !ok {
		t.Fatal("expected tracker to contain client-a after inbound packet")
	}
	if !lastAlive.After(oldAlive) {
		t.Fatalf("LastAlive() = %s, want after %s", lastAlive, oldAlive)
	}
}

func TestUpdateClientAliveTimeUpdatesLocalTracker(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	tracker := clientalive.NewTracker()
	c := NewClient(
		&bufferConn{},
		WithKeepAliveTracker(tracker),
		WithKeepAliveTime(time.Second),
	)
	c.ID = "client-a"
	c.ownerToken = "owner-a"
	c.keepAlive = time.Second

	NewClientHandler(c).UpdateClientAliveTime()

	lastAlive, ok := tracker.LastAlive("client-a")
	if !ok {
		t.Fatal("expected tracker to contain client-a")
	}
	if lastAlive.IsZero() {
		t.Fatal("expected tracker last alive to be set")
	}
	expireAt, ok := tracker.ExpireAt("client-a")
	if !ok {
		t.Fatal("expected tracker expire time to be set")
	}
	if !expireAt.Equal(lastAlive.Add(1500 * time.Millisecond)) {
		t.Fatalf("ExpireAt() = %s, want %s", expireAt, lastAlive.Add(1500*time.Millisecond))
	}
}

func TestCleanupKeepAliveDoesNotDeleteNewOwnerEntry(t *testing.T) {
	tracker := clientalive.NewTracker()
	base := time.Now()
	tracker.Update("client-a", "new-owner", base, time.Second)

	oldClient := NewClient(
		&bufferConn{},
		WithKeepAliveTracker(tracker),
	)
	oldClient.ID = "client-a"
	oldClient.ownerToken = "old-owner"

	oldClient.cleanupKeepAlive(context.Background())

	if _, ok := tracker.LastAlive("client-a"); !ok {
		t.Fatal("expected keepalive tracker to preserve new owner entry")
	}
	got := tracker.ScanExpired(base.Add(2 * time.Second))
	if len(got) != 1 || got[0].ClientID != "client-a" || got[0].OwnerToken != "new-owner" {
		t.Fatalf("ScanExpired() = %v, want preserved new owner entry", got)
	}
}

func TestMQTT5KeepAliveUsesServerKeepAliveWhenConfigured(t *testing.T) {
	got := mqtt5KeepAlive(
		&packets.Connect{KeepAlive: 60},
		&config.ConnectAckProperty{ServerKeepAlive: 3},
	)

	if got != 3*time.Second {
		t.Fatalf("expected negotiated keepalive to use server value, got %s", got)
	}
}

func TestKeepAliveExpiredUsesOneAndHalfTimesNegotiatedKeepAlive(t *testing.T) {
	now := time.Unix(100, 0)
	c := NewClient(&bufferConn{})
	c.keepAlive = time.Second
	c.aliveTime.Store(now.Add(-1600 * time.Millisecond))

	if !c.KeepAliveExpired(now) {
		t.Fatalf("expected client to expire after more than 1.5x keepalive")
	}

	c.aliveTime.Store(now.Add(-1400 * time.Millisecond))
	if c.KeepAliveExpired(now) {
		t.Fatalf("expected client to remain alive before 1.5x keepalive")
	}
}

func TestDisconnectForKeepAliveTimeoutCarriesReasonCode(t *testing.T) {
	cp := DisconnectForKeepAliveTimeout(2*time.Second, time.Second)
	disc, ok := cp.Content.(*packets.Disconnect)
	if !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	if disc.ReasonCode != packets.DisconnectKeepAliveTimeout {
		t.Fatalf("expected reason code 0x8D, got 0x%X", disc.ReasonCode)
	}
	if disc.Properties == nil || disc.Properties.ReasonString == "" {
		t.Fatal("expected ReasonString to be populated for diagnostics")
	}
}

func TestDisconnectForServerShuttingDownCarriesReasonCode(t *testing.T) {
	cp := DisconnectForServerShuttingDown()
	disc, ok := cp.Content.(*packets.Disconnect)
	if !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	if disc.ReasonCode != packets.DisconnectServerShuttingDown {
		t.Fatalf("expected reason code 0x8B, got 0x%X", disc.ReasonCode)
	}
}
