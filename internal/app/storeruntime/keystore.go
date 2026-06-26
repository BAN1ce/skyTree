package storeruntime

import (
	"fmt"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/cluster/healthcheck"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/cluster/raftcfg"
	"github.com/BAN1ce/skyTree/pkg/fsutil"
	badger2 "github.com/BAN1ce/skyTree/pkg/storage/keystore/badger"
	keystore2 "github.com/BAN1ce/skyTree/pkg/storage/keystore/raftstore"
	"github.com/BAN1ce/skyTree/pkg/storage/keystore/redis"
	"github.com/dgraph-io/badger"
	config2 "github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/statemachine"
)

// BuildKeyStore 根据单机或集群模式构建 broker 使用的 keystore。
func BuildKeyStore(cfg config.AppConfig, cluster *raft2.Cluster) (store.KeyStore, error) {
	badgerPath := cfg.Storage.Badger.Path + "/" + fmt.Sprintf("%d", cfg.Cluster.LocalNodeID)

	if !cfg.Cluster.Enable {
		return CreateSingleNodeKeyStore(cfg.Storage, badgerPath)
	}

	if cluster == nil {
		return nil, fmt.Errorf("cluster is nil in cluster mode")
	}

	logger.Logger.Info().Msg("Creating Badger key store for cluster mode")
	if err := fsutil.CreateDir(badgerPath); err != nil {
		return nil, fmt.Errorf("create badger dir failed: %w", err)
	}

	option := badger.DefaultOptions(badgerPath)
	option.NumCompactors = 20
	option.SyncWrites = false
	keyStore, err := badger2.NewBadger(option)
	if err != nil {
		return nil, fmt.Errorf("create badger key store failed: %w", err)
	}

	clusterConfig := raftcfg.RaftConfig(cfg.Cluster)
	keyStoreDescriptor, err := clusterruntime.ClusterDescriptorByID(raft2.ClusterIDKeyStore)
	if err != nil {
		return nil, fmt.Errorf("resolve key store cluster descriptor failed: %w", err)
	}
	clusterConfig.ClusterID = keyStoreDescriptor.ClusterID

	if err := cluster.RegisterStateMachines(map[config2.Config]statemachine.IStateMachine{
		clusterConfig: healthcheck.NewHealthCheckWrapper(keystore2.NewStateMachine(keyStore)),
	}); err != nil {
		return nil, fmt.Errorf("register key store state machine failed: %w", err)
	}

	return keystore2.NewKeyStoreCluster(
		clusterruntime.NewBusinessRaftClient(clusterConfig.ClusterID, cluster, cfg.Cluster),
	), nil
}

// CreateSingleNodeKeyStore 按单机存储配置创建本地或 Redis keystore。
func CreateSingleNodeKeyStore(storeConfig config.Store, badgerPath string) (store.KeyStoreWithBackup, error) {
	switch storeConfig.Default {
	case config.KeyStoreTypeRedis:
		logger.Logger.Info().Str("type", config.KeyStoreTypeRedis).Msg("Creating Redis key store")
		return redis.NewRedis(storeConfig.Redis), nil

	case config.KeyStoreTypeBadger:
		logger.Logger.Info().Str("type", config.KeyStoreTypeBadger).Msg("Creating Badger key store")
		if err := fsutil.CreateDir(badgerPath); err != nil {
			return nil, fmt.Errorf("create badger directory failed: %w", err)
		}
		option := badger.DefaultOptions(badgerPath)
		store, err := badger2.NewBadger(option)
		if err != nil {
			return nil, fmt.Errorf("create badger key store failed: %w", err)
		}
		return store, nil

	default:
		return nil, fmt.Errorf(
			"unsupported storage.driver %q (supported: %s, %s)",
			storeConfig.Default,
			config.KeyStoreTypeBadger,
			config.KeyStoreTypeRedis,
		)
	}
}
