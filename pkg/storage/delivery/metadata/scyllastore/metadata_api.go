package scyllastore

import (
	"context"

	"github.com/BAN1ce/skyTree/config"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
)

// DeliveryMetadataStore stores delivery metadata (tasks/cursors) in Scylla/Cassandra.
type DeliveryMetadataStore = DeliveryQueueStore

func NewDeliveryMetadataStore(cfg config.Cassandra, opts ...Option) (*DeliveryMetadataStore, error) {
	return NewDeliveryMetadataStoreWithContext(context.Background(), cfg, opts...)
}

func NewDeliveryMetadataStoreWithContext(ctx context.Context, cfg config.Cassandra, opts ...Option) (*DeliveryMetadataStore, error) {
	return NewDeliveryQueueStoreWithContext(ctx, cfg, opts...)
}

var _ brokerstore.DeliveryQueueStore = (*DeliveryMetadataStore)(nil)
