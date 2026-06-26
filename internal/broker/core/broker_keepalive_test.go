package core

import (
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/clientalive"
)

func TestBrokerKeepAliveScanIntervalUsesConfig(t *testing.T) {
	b := &Broker{config: brokerConfigSet{broker: config.Broker{KeepAliveScanInterval: 250 * time.Millisecond}}}
	if got := b.keepAliveScanInterval(); got != 250*time.Millisecond {
		t.Fatalf("keepAliveScanInterval() = %s, want 250ms", got)
	}
}

func TestBrokerKeepAliveScanIntervalDefaultsWhenUnset(t *testing.T) {
	b := &Broker{}
	if got := b.keepAliveScanInterval(); got != defaultBrokerKeepAliveScanInterval {
		t.Fatalf("keepAliveScanInterval() = %s, want %s", got, defaultBrokerKeepAliveScanInterval)
	}
}

func TestDeleteUnAliveClientSkipsStaleOwnerToken(t *testing.T) {
	tracker := clientalive.NewTracker()
	base := time.Now().Add(-time.Minute)
	tracker.Update("client-a", "old-owner", base, time.Second)

	currentClient := client.NewClient(nil)
	currentClient.ID = "client-a"
	manager := client.NewManager()
	manager.AddClient("client-a", currentClient)

	b := &Broker{
		clients: brokerClientResources{
			manager:          manager,
			keepAliveTracker: tracker,
		},
	}

	b.deleteUnAliveClient()

	got, ok := manager.ReadClient("client-a")
	if !ok {
		t.Fatal("expected current client to remain online")
	}
	if got != currentClient {
		t.Fatal("expected manager to keep current client instance")
	}
}
