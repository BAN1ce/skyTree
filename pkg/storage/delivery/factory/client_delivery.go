package factory

import (
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/config"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/storage/delivery/combined"
	messagepayload_badger "github.com/BAN1ce/skyTree/pkg/storage/delivery/messagepayload/badger"
	messagepayload_scyllastore "github.com/BAN1ce/skyTree/pkg/storage/delivery/messagepayload/scyllastore"
	metadata_badgerstore "github.com/BAN1ce/skyTree/pkg/storage/delivery/metadata/badgerstore"
	metadata_scyllastore "github.com/BAN1ce/skyTree/pkg/storage/delivery/metadata/scyllastore"
)

func BuildMessagePayloadStore(cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.MessagePayloadStore, error) {
	return BuildMessagePayloadStoreWithContext(context.Background(), cfg, cluster)
}

func BuildMessagePayloadStoreWithContext(ctx context.Context, cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.MessagePayloadStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = cluster
	spec, err := cfg.ResolveClientDeliverySpec()
	if err != nil {
		return nil, err
	}
	ttl := messagePayloadTTL(cfg)
	switch spec.PayloadType {
	case config.ClientDeliveryPayloadTypeScylla:
		return messagepayload_scyllastore.NewMessagePayloadStoreWithContext(ctx, cfg.Storage.Cassandra, ttl)
	case config.ClientDeliveryPayloadTypeSingleNodeBadger:
		return messagepayload_badger.NewMessagePayloadStore(cfg.Storage.Badger.Path, cfg.Cluster.LocalNodeID, ttl)
	default:
		return nil, fmt.Errorf(
			"unsupported payload store type=%q (supported: %s, %s)",
			spec.PayloadType,
			config.ClientDeliveryPayloadTypeSingleNodeBadger,
			config.ClientDeliveryPayloadTypeScylla,
		)
	}
}

func messagePayloadTTL(cfg config.AppConfig) time.Duration {
	if cfg.Storage.MessageExpired <= 0 {
		return 0
	}
	ttl := time.Duration(cfg.Storage.MessageExpired) * 24 * time.Hour
	sessionMax := cfg.Broker.Limits.SessionExpiryMaxSeconds
	if sessionMax == 0 {
		return ttl
	}
	sessionTTL := time.Duration(sessionMax) * time.Second
	if ttl <= 0 || ttl < sessionTTL {
		return sessionTTL
	}
	return ttl
}

func BuildDeliveryMetadataStore(cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.DeliveryQueueStore, error) {
	return BuildDeliveryMetadataStoreWithContext(context.Background(), cfg, cluster)
}

func BuildDeliveryMetadataStoreWithContext(ctx context.Context, cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.DeliveryQueueStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = cluster
	spec, err := cfg.ResolveClientDeliverySpec()
	if err != nil {
		return nil, err
	}
	switch spec.QueueType {
	case config.ClientDeliveryQueueTypeScylla:
		return metadata_scyllastore.NewDeliveryMetadataStoreWithContext(
			ctx,
			cfg.Storage.Cassandra,
			metadata_scyllastore.WithBucketDuration(cfg.Storage.DeliveryQueue.BucketDuration),
		)
	case config.ClientDeliveryQueueTypeSingleNodeBadger:
		return metadata_badgerstore.NewLocalDeliveryMetadataStore(cfg.Storage.Badger.Path, cfg.Cluster.LocalNodeID)
	default:
		return nil, fmt.Errorf(
			"unsupported delivery queue store type=%q (supported: %s, %s)",
			spec.QueueType,
			config.ClientDeliveryQueueTypeSingleNodeBadger,
			config.ClientDeliveryQueueTypeScylla,
		)
	}
}

func BuildClientDeliveryStore(cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.ClientDeliveryStore, error) {
	return BuildClientDeliveryStoreWithContext(context.Background(), cfg, cluster)
}

func BuildClientDeliveryStoreWithContext(ctx context.Context, cfg config.AppConfig, cluster *raft2.Cluster) (brokerstore.ClientDeliveryStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Resolve first so misconfiguration is rejected early with a clear message.
	if _, err := cfg.ResolveClientDeliverySpec(); err != nil {
		return nil, err
	}

	metadataStore, err := BuildDeliveryMetadataStoreWithContext(ctx, cfg, cluster)
	if err != nil {
		return nil, err
	}
	payload, err := BuildMessagePayloadStoreWithContext(ctx, cfg, cluster)
	if err != nil {
		return nil, err
	}

	s, err := combined.NewClientDeliveryStore(metadataStore, payload)
	if err != nil {
		return nil, err
	}
	return s, nil
}
