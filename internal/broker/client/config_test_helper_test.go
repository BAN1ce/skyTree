package client

import (
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
)

func mustLoadConfigForTest(t *testing.T) config.AppConfig {
	t.Helper()
	cfg, err := config.Load("../../../etc/config.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func clientConfigForTest(brokerCfg config.Broker) Config {
	return Config{
		BrokerConfig:         brokerCfg,
		BrokerConfigResolved: true,
		WriteTimeout:         200 * time.Millisecond,
	}
}
