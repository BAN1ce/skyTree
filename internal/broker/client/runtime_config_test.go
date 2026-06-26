package client

import (
	"testing"

	"github.com/BAN1ce/skyTree/config"
)

func TestBrokerRuntimeConfig_PartialConfigKeepsDefaultsForUnsetCapabilities(t *testing.T) {
	c := NewClient(nil, WithConfig(Config{
		BrokerConfig: config.Broker{
			ConnectAckProperty: config.ConnectAckProperty{
				TopicAliasMaximum: 7,
			},
		},
	}))

	cfg := c.brokerRuntimeConfig()
	if cfg.ConnectAckProperty.MaxQos != 2 {
		t.Fatalf("MaxQos: want 2, got %d", cfg.ConnectAckProperty.MaxQos)
	}
	if cfg.ConnectAckProperty.RetainAvailable != 1 {
		t.Fatalf("RetainAvailable: want 1, got %d", cfg.ConnectAckProperty.RetainAvailable)
	}
	if !cfg.ConnectAckProperty.WildcardSubscriptionAvailable {
		t.Fatal("WildcardSubscriptionAvailable: want true")
	}
	if !cfg.ConnectAckProperty.SharedSubscriptionAvailable {
		t.Fatal("SharedSubscriptionAvailable: want true")
	}
	if cfg.ConnectAckProperty.TopicAliasMaximum != 7 {
		t.Fatalf("TopicAliasMaximum: want 7, got %d", cfg.ConnectAckProperty.TopicAliasMaximum)
	}
}

func TestBrokerRuntimeConfig_ResolvedConfigPreservesExplicitCapabilityValues(t *testing.T) {
	base := defaultBrokerRuntimeConfig()
	base.ConnectAckProperty.MaxQos = 0
	base.ConnectAckProperty.RetainAvailable = 0
	base.ConnectAckProperty.WildcardSubscriptionAvailable = false
	base.ConnectAckProperty.SharedSubscriptionAvailable = false

	c := NewClient(nil, WithConfig(Config{
		BrokerConfig:         base,
		BrokerConfigResolved: true,
	}))

	cfg := c.brokerRuntimeConfig()
	if cfg.ConnectAckProperty.MaxQos != 0 {
		t.Fatalf("MaxQos: want 0, got %d", cfg.ConnectAckProperty.MaxQos)
	}
	if cfg.ConnectAckProperty.RetainAvailable != 0 {
		t.Fatalf("RetainAvailable: want 0, got %d", cfg.ConnectAckProperty.RetainAvailable)
	}
	if cfg.ConnectAckProperty.WildcardSubscriptionAvailable {
		t.Fatal("WildcardSubscriptionAvailable: want false")
	}
	if cfg.ConnectAckProperty.SharedSubscriptionAvailable {
		t.Fatal("SharedSubscriptionAvailable: want false")
	}
}
