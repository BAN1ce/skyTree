package storeruntime

import (
	"context"

	"github.com/BAN1ce/skyTree/config"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/storage/delivery/factory"
)

// BuildClientDeliveryStore 根据配置创建客户端投递队列存储。
func BuildClientDeliveryStore(
	ctx context.Context,
	cfg config.AppConfig,
	cluster *raft2.Cluster,
) (store.ClientDeliveryStore, error) {
	return factory.BuildClientDeliveryStoreWithContext(ctx, cfg, cluster)
}
