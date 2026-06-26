package scyllastore

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/config"
	messagepayload_cassandra "github.com/BAN1ce/skyTree/pkg/storage/delivery/messagepayload/cassandra"
)

// MessagePayloadStore stores serialized message bytes (data plane) on ScyllaDB/CQL.
//
// During migration this reuses the Cassandra-compatible implementation.
type MessagePayloadStore = messagepayload_cassandra.MessagePayloadStore

func NewMessagePayloadStore(cfg config.Cassandra, ttl time.Duration) (*MessagePayloadStore, error) {
	return NewMessagePayloadStoreWithContext(context.Background(), cfg, ttl)
}

func NewMessagePayloadStoreWithContext(ctx context.Context, cfg config.Cassandra, ttl time.Duration) (*MessagePayloadStore, error) {
	return messagepayload_cassandra.NewMessagePayloadStoreWithContext(ctx, cfg, ttl)
}
