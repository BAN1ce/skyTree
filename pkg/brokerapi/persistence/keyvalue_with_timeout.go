package persistence

import (
	"context"
	"time"
)

type KeyValueStoreWithTimeout struct {
	KeyStore
	baseCtx context.Context
	timeout time.Duration
}

func NewKeyValueStoreWithTimeout(store KeyStore, timeout time.Duration) *KeyValueStoreWithTimeout {
	return NewKeyValueStoreWithTimeoutFromContext(context.Background(), store, timeout)
}

func NewKeyValueStoreWithTimeoutFromContext(parent context.Context, store KeyStore, timeout time.Duration) *KeyValueStoreWithTimeout {
	if parent == nil {
		parent = context.Background()
	}
	return &KeyValueStoreWithTimeout{
		KeyStore: store,
		baseCtx:  parent,
		timeout:  timeout,
	}
}

func (k *KeyValueStoreWithTimeout) getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(k.baseCtx, k.timeout)
}

func (k *KeyValueStoreWithTimeout) DefaultPutKey(key, value []byte) error {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.PutKey(ctx, key, value)
}

func (k *KeyValueStoreWithTimeout) DefaultReadKey(key []byte) ([]byte, bool, error) {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.ReadKey(ctx, key)
}

func (k *KeyValueStoreWithTimeout) DefaultHSet(key []byte, filed [][]byte) error {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.HSet(ctx, key, filed)
}

func (k *KeyValueStoreWithTimeout) DefaultHGet(key, filed []byte) ([]byte, bool, error) {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.HGet(ctx, key, filed)
}

func (k *KeyValueStoreWithTimeout) DefaultHDel(key []byte, filed [][]byte) error {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.HDel(ctx, key, filed)
}

func (k *KeyValueStoreWithTimeout) DefaultHPrefix(key, prefix []byte) (map[string]string, error) {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.HPrefix(ctx, key, prefix)
}

func (k *KeyValueStoreWithTimeout) DefaultDeleteHash(key []byte) error {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.DeleteHash(ctx, key)
}

func (k *KeyValueStoreWithTimeout) DefaultDeleteKey(key []byte) error {
	ctx, cancel := k.getCtx()
	defer cancel()
	return k.KeyStore.DeleteKey(ctx, key)
}
