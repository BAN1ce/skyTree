package broker

import (
	"context"
	"time"
)

type KeyValueStore interface {
	PutKey(ctx context.Context, key, value string) error
	ReadKey(ctx context.Context, key string) (string, bool, error)
	DeleteKey(ctx context.Context, key string) error
	ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error)
	DeletePrefixKey(ctx context.Context, prefix string) error
}

type KeyValueStoreWithTimeout struct {
	KeyValueStore
	timeout time.Duration
}

func NewKeyValueStoreWithTimout(store KeyValueStore, timeout time.Duration) *KeyValueStoreWithTimeout {
	return &KeyValueStoreWithTimeout{
		KeyValueStore: store,
		timeout:       timeout,
	}
}

func (s *KeyValueStoreWithTimeout) getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), s.timeout)
}

func (s *KeyValueStoreWithTimeout) DefaultPutKey(key, value string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyValueStore.PutKey(ctx, key, value)

}

func (s *KeyValueStoreWithTimeout) DefaultReadKey(key string) (string, bool, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyValueStore.ReadKey(ctx, key)

}

func (s *KeyValueStoreWithTimeout) DefaultDeleteKey(key string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyValueStore.DeleteKey(ctx, key)
}

func (s *KeyValueStoreWithTimeout) DefaultReadPrefixKey(prefix string) (map[string]string, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyValueStore.ReadPrefixKey(ctx, prefix)
}
