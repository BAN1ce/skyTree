package badger

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/fsutil"
	badgerdb "github.com/dgraph-io/badger"
	"github.com/google/uuid"
)

// MessagePayloadStore is a local (non-Raft) payload store backed by Badger.
//
// It is intended for single-node mode or as a lightweight local backend.
type MessagePayloadStore struct {
	db  *badgerdb.DB
	ttl time.Duration
}

var _ store.MessagePayloadStore = (*MessagePayloadStore)(nil)

func NewMessagePayloadStore(basePath string, nodeID uint64, ttl time.Duration) (*MessagePayloadStore, error) {
	path := filepath.Join(basePath, fmt.Sprintf("%d", nodeID), "payload_local")
	if err := fsutil.CreateDir(path); err != nil {
		return nil, err
	}
	opt := badgerdb.DefaultOptions(path)
	opt.SyncWrites = false
	db, err := badgerdb.Open(opt)
	if err != nil {
		return nil, err
	}
	return &MessagePayloadStore{db: db, ttl: ttl}, nil
}

func (s *MessagePayloadStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *MessagePayloadStore) SaveMessagePayload(ctx context.Context, record store.MessagePayloadRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("badger db is nil")
	}
	if record.MessageID == uuid.Nil {
		return fmt.Errorf("messageID is empty")
	}

	key := payloadKey(record.MessageID.String())
	return s.db.Update(func(txn *badgerdb.Txn) error {
		e := badgerdb.NewEntry(key, record.Payload)
		if s.ttl > 0 {
			e = e.WithTTL(s.ttl)
		}
		return txn.SetEntry(e)
	})
}

func (s *MessagePayloadStore) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("badger db is nil")
	}
	if messageID == uuid.Nil {
		return nil, fmt.Errorf("messageID is empty")
	}

	key := payloadKey(messageID.String())
	var out []byte
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(key)
		if err == badgerdb.ErrKeyNotFound {
			return store.ErrMessagePayloadNotFound
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			out = append([]byte(nil), val...)
			return nil
		})
	})
	return out, err
}

func payloadKey(messageID string) []byte {
	// Align with payload_raft key format for consistent semantics.
	// Format: p/<uuid>
	return append([]byte("p/"), []byte(messageID)...)
}
