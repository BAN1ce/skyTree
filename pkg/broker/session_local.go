package broker

import (
	"context"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/Tree/state/store"
)

type LocalKeyValueStore struct {
	session *store.State
}

func NewLocalKeyValueStore() *LocalKeyValueStore {
	return &LocalKeyValueStore{
		session: store.NewState(),
	}
}

func (l *LocalKeyValueStore) PutKey(ctx context.Context, key, value string) error {
	_, err := l.session.HandlePutKeyRequest(&proto.PutKeyRequest{
		Key:   key,
		Value: value,
	})
	return err
}

func (l *LocalKeyValueStore) ReadKey(ctx context.Context, key string) (string, bool, error) {
	return l.session.ReadKey(key)
}

func (l *LocalKeyValueStore) DeleteKey(ctx context.Context, key string) error {
	_, err := l.session.HandleDeleteKeyRequest(&proto.DeleteKeyRequest{
		Key: key,
	})
	return err
}

func (l *LocalKeyValueStore) ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error) {
	return l.session.ReadWithPrefix(prefix)
}
