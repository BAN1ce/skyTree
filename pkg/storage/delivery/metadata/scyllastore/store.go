package scyllastore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/config"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/gocql/gocql"
)

const defaultBucketDuration = time.Hour

type StoreOptions struct {
	BucketDuration time.Duration
}

type Option func(*StoreOptions)

type DeliveryQueueStore struct {
	session        cqlSession
	bucketDuration time.Duration
}

var _ brokerstore.DeliveryQueueStore = (*DeliveryQueueStore)(nil)
var _ brokerstore.SharedSubscriptionStore = (*DeliveryQueueStore)(nil)

func WithBucketDuration(bucketDuration time.Duration) Option {
	return func(o *StoreOptions) {
		if bucketDuration > 0 {
			o.BucketDuration = bucketDuration
		}
	}
}

func NewDeliveryQueueStore(cfg config.Cassandra, opts ...Option) (*DeliveryQueueStore, error) {
	return NewDeliveryQueueStoreWithContext(context.Background(), cfg, opts...)
}

func NewDeliveryQueueStoreWithContext(ctx context.Context, cfg config.Cassandra, opts ...Option) (*DeliveryQueueStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("scylla hosts is empty")
	}
	if cfg.Keyspace == "" {
		return nil, fmt.Errorf("scylla keyspace is empty")
	}
	if cfg.Port == 0 {
		cfg.Port = 9042
	}

	cluster := gocql.NewCluster(cfg.Hosts...)
	cluster.Port = cfg.Port
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = parseConsistency(cfg.Consistency)
	cluster.Timeout = cfg.Timeout
	cluster.ConnectTimeout = cfg.ConnectTimeout
	if cfg.NumConns > 0 {
		cluster.NumConns = cfg.NumConns
	}
	if cfg.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.Username,
			Password: cfg.Password,
		}
	}
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("create scylla session: %w", err)
	}
	store := newStoreWithSession(&gocqlSession{session: session}, newStoreOptions(opts...))
	if cfg.AutoCreateTable {
		schemaCtx, cancel := context.WithTimeout(ctx, schemaInitTimeout(cfg))
		defer cancel()
		if err := store.EnsureDeliverySchema(schemaCtx); err != nil {
			session.Close()
			return nil, fmt.Errorf("ensure scylla delivery schema: %w", err)
		}
	}
	return store, nil
}

func newStoreOptions(opts ...Option) StoreOptions {
	options := StoreOptions{BucketDuration: defaultBucketDuration}
	for _, opt := range opts {
		opt(&options)
	}
	if options.BucketDuration <= 0 {
		options.BucketDuration = defaultBucketDuration
	}
	return options
}

func newStoreWithSession(session cqlSession, options StoreOptions) *DeliveryQueueStore {
	if options.BucketDuration <= 0 {
		options.BucketDuration = defaultBucketDuration
	}
	return &DeliveryQueueStore{
		session:        session,
		bucketDuration: options.BucketDuration,
	}
}

func (s *DeliveryQueueStore) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	s.session.Close()
	return nil
}

func (s *DeliveryQueueStore) EnsureDeliverySchema(ctx context.Context) error {
	if s == nil || s.session == nil {
		return fmt.Errorf("scylla session is nil")
	}
	for _, stmt := range deliverySchemaStatements {
		if err := s.session.Query(stmt).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("ensure delivery schema: %w", err)
		}
	}
	return nil
}

func (s *DeliveryQueueStore) EnsureSchema(ctx context.Context) error {
	return s.EnsureDeliverySchema(ctx)
}

func bucketStartNano(ts time.Time, bucketDuration time.Duration) int64 {
	if bucketDuration <= 0 {
		bucketDuration = defaultBucketDuration
	}
	return ts.Truncate(bucketDuration).UnixNano()
}

func schemaInitTimeout(cfg config.Cassandra) time.Duration {
	if cfg.ConnectTimeout > 0 {
		return cfg.ConnectTimeout
	}
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return 10 * time.Second
}

func parseConsistency(v string) gocql.Consistency {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "ANY":
		return gocql.Any
	case "ONE":
		return gocql.One
	case "TWO":
		return gocql.Two
	case "THREE":
		return gocql.Three
	case "QUORUM":
		return gocql.Quorum
	case "ALL":
		return gocql.All
	case "LOCAL_QUORUM":
		return gocql.LocalQuorum
	case "EACH_QUORUM":
		return gocql.EachQuorum
	case "LOCAL_ONE":
		return gocql.LocalOne
	default:
		return gocql.LocalQuorum
	}
}

var deliverySchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS delivery_client_state (
  client_id text PRIMARY KEY,
  generation bigint,
  last_ts_nano bigint,
  last_task_id uuid,
  updated_at timestamp
)`,
	`CREATE TABLE IF NOT EXISTS delivery_client_buckets (
  client_id text,
  generation bigint,
  bucket_start_nano bigint,
  PRIMARY KEY ((client_id, generation), bucket_start_nano)
) WITH CLUSTERING ORDER BY (bucket_start_nano ASC)`,
	`CREATE TABLE IF NOT EXISTS delivery_tasks_by_client_bucket (
  client_id text,
  generation bigint,
  bucket_start_nano bigint,
  ts_nano bigint,
  task_id uuid,
  message_id uuid,
  delivery_qos int,
  subscription_ids list<int>,
  no_local boolean,
  retain_as_published boolean,
  share_group text,
  shared_task_id uuid,
  PRIMARY KEY ((client_id, generation, bucket_start_nano), ts_nano, task_id)
) WITH CLUSTERING ORDER BY (ts_nano ASC, task_id ASC)`,
	`CREATE TABLE IF NOT EXISTS delivery_task_dedupe_by_client (
  client_id text,
  generation bigint,
  message_id uuid,
  bucket_start_nano bigint,
  ts_nano bigint,
  task_id uuid,
  PRIMARY KEY ((client_id, generation), message_id)
)`,
	`CREATE TABLE IF NOT EXISTS share_group_cursor (
  share_group text PRIMARY KEY,
  last_processed_ts_nano bigint,
  last_processed_task_id uuid,
  leader_node_id bigint,
  last_renewal timestamp
)`,
	`CREATE TABLE IF NOT EXISTS share_group_buckets (
  share_group text,
  status text,
  bucket_start_nano bigint,
  PRIMARY KEY ((share_group, status), bucket_start_nano)
) WITH CLUSTERING ORDER BY (bucket_start_nano ASC)`,
	`CREATE TABLE IF NOT EXISTS share_group_tasks_by_status_bucket (
  share_group text,
  status text,
  bucket_start_nano bigint,
  ts_nano bigint,
  task_id uuid,
  topic_filter text,
  message_id uuid,
  delivery_qos int,
  subscription_ids text,
  winner_no_local boolean,
  winner_rap boolean,
  processed_by_node bigint,
  processed_at_nano bigint,
  rollback_reason text,
  PRIMARY KEY ((share_group, status, bucket_start_nano), ts_nano, task_id)
) WITH CLUSTERING ORDER BY (ts_nano ASC, task_id ASC)`,
	`CREATE TABLE IF NOT EXISTS share_group_task_by_id (
  share_group text,
  task_id uuid,
  status text,
  bucket_start_nano bigint,
  ts_nano bigint,
  topic_filter text,
  message_id uuid,
  delivery_qos int,
  subscription_ids text,
  winner_no_local boolean,
  winner_rap boolean,
  processed_by_node bigint,
  processed_at_nano bigint,
  rollback_reason text,
  PRIMARY KEY (share_group, task_id)
)`,
	`CREATE TABLE IF NOT EXISTS share_group_task_by_message (
  share_group text,
  message_id uuid,
  status text,
  task_id uuid,
  PRIMARY KEY ((share_group, message_id), status, task_id)
)`,
}

func statusString(status sharedsubscription.TaskStatus) string {
	if status == "" {
		return string(sharedsubscription.TaskStatusPending)
	}
	return string(status)
}
