package storeruntime

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

// Runtime 保存 broker 启动后需要共享的存储相关依赖。
type Runtime struct {
	KeyStore                store.KeyStore
	ClientDeliveryStore     store.ClientDeliveryStore
	DeliveryTaskStore       delivery.TaskStore
	DeliveryCursorStore     delivery.CursorStore
	SharedSubscriptionStore store.SharedSubscriptionStore
}

// Build 构建 keystore、投递存储和投递任务访问对象。
func Build(ctx context.Context, cfg config.AppConfig, cluster *raft2.Cluster) (*Runtime, error) {
	keyStore, err := BuildKeyStore(cfg, cluster)
	if err != nil {
		return nil, fmt.Errorf("build keystore failed: %w", err)
	}

	clientDeliveryStore, err := BuildClientDeliveryStore(ctx, cfg, cluster)
	if err != nil {
		return nil, fmt.Errorf("build client delivery store failed: %w", err)
	}

	deliveryTaskStore, err := delivery.NewClientDeliveryTaskStore(clientDeliveryStore)
	if err != nil {
		return nil, fmt.Errorf("init delivery task store failed: %w", err)
	}
	deliveryCursorStore, err := delivery.NewClientDeliveryCursorStore(clientDeliveryStore)
	if err != nil {
		return nil, fmt.Errorf("init delivery cursor store failed: %w", err)
	}

	schemaEnsurer, ok := interface{}(deliveryTaskStore).(delivery.SchemaEnsurer)
	if !ok {
		return nil, fmt.Errorf("delivery task store does not implement SchemaEnsurer")
	}
	if err := schemaEnsurer.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensure delivery schema failed: %w", err)
	}

	var sharedSubscriptionStore store.SharedSubscriptionStore
	if ss, ok := clientDeliveryStore.(store.SharedSubscriptionStore); ok {
		sharedSubscriptionStore = ss
	}

	return &Runtime{
		KeyStore:                keyStore,
		ClientDeliveryStore:     clientDeliveryStore,
		DeliveryTaskStore:       deliveryTaskStore,
		DeliveryCursorStore:     deliveryCursorStore,
		SharedSubscriptionStore: sharedSubscriptionStore,
	}, nil
}
