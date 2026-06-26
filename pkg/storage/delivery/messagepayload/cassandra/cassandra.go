package cassandra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

type MessagePayloadStore struct {
	session *gocql.Session
	cfg     config.Cassandra
	ttl     time.Duration
}

var _ brokerstore.MessagePayloadStore = (*MessagePayloadStore)(nil)

func NewMessagePayloadStore(cfg config.Cassandra, ttl time.Duration) (*MessagePayloadStore, error) {
	return NewMessagePayloadStoreWithContext(context.Background(), cfg, ttl)
}

func NewMessagePayloadStoreWithContext(ctx context.Context, cfg config.Cassandra, ttl time.Duration) (*MessagePayloadStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("cassandra hosts is empty")
	}
	if cfg.Keyspace == "" {
		return nil, fmt.Errorf("cassandra keyspace is empty")
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
	// Use token-aware host selection.
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	s := &MessagePayloadStore{session: session, cfg: cfg, ttl: ttl}
	if cfg.AutoCreateTable {
		schemaCtx, cancel := context.WithTimeout(ctx, schemaInitTimeout(cfg))
		defer cancel()
		if err := s.ensureSchema(schemaCtx); err != nil {
			session.Close()
			return nil, err
		}
	}
	logger.Logger.Info().Strs("hosts", cfg.Hosts).Str("keyspace", cfg.Keyspace).Msg("Cassandra message payload store initialized")
	return s, nil
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

func (s *MessagePayloadStore) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	s.session.Close()
	return nil
}

func (s *MessagePayloadStore) SaveMessagePayload(ctx context.Context, record brokerstore.MessagePayloadRecord) error {
	if s == nil || s.session == nil {
		return fmt.Errorf("cassandra session is nil")
	}
	if record.MessageID == uuid.Nil {
		return fmt.Errorf("messageID is empty")
	}
	id, err := gocql.ParseUUID(record.MessageID.String())
	if err != nil {
		return err
	}
	stmt, ttlSeconds := insertMessagePayloadStatement(s.ttl)
	args := []any{id, record.CreatedAt.UTC(), record.PublishTopic, record.PublisherClientID, record.Payload}
	if ttlSeconds > 0 {
		args = append(args, ttlSeconds)
	}
	q := s.session.Query(stmt, args...).WithContext(ctx)
	return q.Exec()
}

func (s *MessagePayloadStore) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	if s == nil || s.session == nil {
		return nil, fmt.Errorf("cassandra session is nil")
	}
	if messageID == uuid.Nil {
		return nil, fmt.Errorf("messageID is empty")
	}
	id, err := gocql.ParseUUID(messageID.String())
	if err != nil {
		return nil, err
	}
	var payload []byte
	q := s.session.Query(`SELECT payload FROM message_body WHERE message_id = ? LIMIT 1`, id).WithContext(ctx)
	err = q.Scan(&payload)
	if err == gocql.ErrNotFound {
		return nil, brokerstore.ErrMessagePayloadNotFound
	}
	return payload, err
}

func (s *MessagePayloadStore) ensureSchema(ctx context.Context) error {
	if s == nil || s.session == nil {
		return fmt.Errorf("cassandra session is nil")
	}
	// NOTE: table name is fixed to keep the interface stable.
	stmt := `
CREATE TABLE IF NOT EXISTS message_body (
  message_id uuid PRIMARY KEY,
  created_ts timestamp,
  publish_topic text,
  publisher_client_id text,
  payload blob
)`
	return s.session.Query(stmt).WithContext(ctx).Exec()
}

func insertMessagePayloadStatement(ttl time.Duration) (string, int) {
	const insertMessagePayload = `INSERT INTO message_body (message_id, created_ts, publish_topic, publisher_client_id, payload) VALUES (?, ?, ?, ?, ?)`
	if ttl <= 0 {
		return insertMessagePayload, 0
	}
	ttlSeconds := int(ttl / time.Second)
	if ttlSeconds <= 0 {
		ttlSeconds = 1
	}
	return insertMessagePayload + ` USING TTL ?`, ttlSeconds
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
