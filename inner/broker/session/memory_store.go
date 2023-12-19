package session

import (
	"context"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/Tree/state/store"
)

type MemoryKeyValueStore struct {
	session *store.State
}

func NewMemoryKeyValueStore() *MemoryKeyValueStore {
	return &MemoryKeyValueStore{
		session: store.NewState(),
	}
}

func (l *MemoryKeyValueStore) PutKey(ctx context.Context, key, value string) error {
	_, err := l.session.HandlePutKeyRequest(&proto.PutKeyRequest{
		Key:   key,
		Value: value,
	})
	return err
}

func (l *MemoryKeyValueStore) ReadKey(ctx context.Context, key string) (string, bool, error) {
	return l.session.ReadKey(key)
}

func (l *MemoryKeyValueStore) DeleteKey(ctx context.Context, key string) error {
	_, err := l.session.HandleDeleteKeyRequest(&proto.DeleteKeyRequest{
		Key: key,
	})
	return err
}

func (l *MemoryKeyValueStore) ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error) {
	return l.session.ReadWithPrefix(prefix)
}
