package app

import (
	"context"
	"strings"
	"testing"

	"github.com/BAN1ce/skyTree/config"
)

// TestNewAppReturnsErrorWhenBrokerServerConfigIsInvalid 验证 broker 监听配置非法时 NewApp 返回错误。
func TestNewAppReturnsErrorWhenBrokerServerConfigIsInvalid(t *testing.T) {
	ensureTestLogger()

	cfg := config.AppConfig{
		Broker: config.Broker{
			Listen: []string{"mqtt://127.0.0.1:1883"},
			LocalState: config.LocalState{
				DataDir: t.TempDir(),
			},
		},
		Storage: config.Store{
			Default: config.KeyStoreTypeBadger,
			Badger: config.Badger{
				Path: t.TempDir(),
			},
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: config.Cluster{
			Enable:      false,
			LocalNodeID: 1,
		},
	}

	_, err := NewApp(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected NewApp to fail for invalid broker listener protocol")
	}
	if !strings.Contains(err.Error(), "create broker failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
