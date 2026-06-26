package persistence

import (
	"context"
	"io"
	"time"
)

// --- Capability interfaces (smaller building blocks) ---

// KVWriter writes key-value entries.
type KVWriter interface {
	PutKey(ctx context.Context, key, value []byte) error
	DeleteKey(ctx context.Context, key []byte) error
}

// KVReader reads key-value entries.
type KVReader interface {
	ReadKey(ctx context.Context, key []byte) ([]byte, bool, error)
}

// KVStore is the minimal KV store used by most modules (read + write).
type KVStore interface {
	KVWriter
	KVReader
}

// PrefixDeleter deletes all keys with the given prefix.
type PrefixDeleter interface {
	DeletePrefixKey(ctx context.Context, prefix []byte) error
}

// Expirer sets a TTL/expiration for a key.
// NOTE: This is intentionally optional; most callers should not depend on it.
type Expirer interface {
	SetExpired(ctx context.Context, key []byte, duration time.Duration) error
}

// Snapshotter provides backup/restore capability.
type Snapshotter interface {
	Snapshot(writer io.Writer) error
	Recover(reader io.Reader) error
}

type KeyStoreWithBackup interface {
	KeyStore
	Snapshotter
}

// KeyValueStore is a minimal KV+prefix-delete store.
type KeyValueStore interface {
	KVStore
	PrefixDeleter
}

type KeyStore interface {
	KeyValueStore
	HashStore
	io.Closer
}

type HashStore interface {
	HSet(ctx context.Context, key []byte, field [][]byte) error
	HGet(ctx context.Context, key, field []byte) ([]byte, bool, error)
	HDel(ctx context.Context, key []byte, field [][]byte) error
	HGetAll(ctx context.Context, key []byte) (map[string]string, error)
	HPrefix(ctx context.Context, key []byte, prefix []byte) (map[string]string, error)
	DeleteHash(ctx context.Context, key []byte) error
}
