package persistence

import (
	"context"
	"time"
)

// HashStoreWithTimeout wraps a HashStore and applies a default context timeout to each call.
// This avoids forcing callers to depend on the larger KeyStore interface.
type HashStoreWithTimeout struct {
	HashStore
	baseCtx context.Context
	timeout time.Duration
}

func NewHashStoreWithTimeout(store HashStore, timeout time.Duration) *HashStoreWithTimeout {
	return NewHashStoreWithTimeoutFromContext(context.Background(), store, timeout)
}

func NewHashStoreWithTimeoutFromContext(parent context.Context, store HashStore, timeout time.Duration) *HashStoreWithTimeout {
	if parent == nil {
		parent = context.Background()
	}
	return &HashStoreWithTimeout{
		HashStore: store,
		baseCtx:   parent,
		timeout:   timeout,
	}
}

func (h *HashStoreWithTimeout) getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.baseCtx, h.timeout)
}

func (h *HashStoreWithTimeout) DefaultHSet(key []byte, field [][]byte) error {
	ctx, cancel := h.getCtx()
	defer cancel()
	return h.HashStore.HSet(ctx, key, field)
}

func (h *HashStoreWithTimeout) DefaultHGet(key, field []byte) ([]byte, bool, error) {
	ctx, cancel := h.getCtx()
	defer cancel()
	return h.HashStore.HGet(ctx, key, field)
}

func (h *HashStoreWithTimeout) DefaultHDel(key []byte, field [][]byte) error {
	ctx, cancel := h.getCtx()
	defer cancel()
	return h.HashStore.HDel(ctx, key, field)
}

func (h *HashStoreWithTimeout) DefaultHGetAll(key []byte) (map[string]string, error) {
	ctx, cancel := h.getCtx()
	defer cancel()
	return h.HashStore.HGetAll(ctx, key)
}
