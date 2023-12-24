package broker

import (
	"context"
	"time"
)

type KeyStore interface {
	HashStore
	ZSetStore
}
type HashStore interface {
	PutKey(ctx context.Context, key, value string) error
	ReadKey(ctx context.Context, key string) (string, bool, error)
	DeleteKey(ctx context.Context, key string) error
	ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error)
	DeletePrefixKey(ctx context.Context, prefix string) error
}

type ZSetStore interface {
	ZAdd(ctx context.Context, key, member string, score float64) error
	ZDel(ctx context.Context, key, member string) error
	ZRange(ctx context.Context, key string, start, end float64) ([]string, error)
}

type KeyValueStoreWithTimeout struct {
	KeyStore
	timeout time.Duration
}

func NewKeyValueStoreWithTimout(store KeyStore, timeout time.Duration) *KeyValueStoreWithTimeout {
	return &KeyValueStoreWithTimeout{
		KeyStore: store,
		timeout:  timeout,
	}
}

func (s *KeyValueStoreWithTimeout) getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), s.timeout)
}

func (s *KeyValueStoreWithTimeout) DefaultPutKey(key, value string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyStore.PutKey(ctx, key, value)

}

func (s *KeyValueStoreWithTimeout) DefaultReadKey(key string) (string, bool, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyStore.ReadKey(ctx, key)

}

func (s *KeyValueStoreWithTimeout) DefaultDeleteKey(key string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyStore.DeleteKey(ctx, key)
}

func (s *KeyValueStoreWithTimeout) DefaultReadPrefixKey(prefix string) (map[string]string, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.KeyStore.ReadPrefixKey(ctx, prefix)
}
