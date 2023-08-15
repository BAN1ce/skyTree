package broker

import (
	"context"
	"time"
)

type SessionStore interface {
	PutKey(ctx context.Context, key, value string) error
	ReadKey(ctx context.Context, key string) (string, bool, error)
	DeleteKey(ctx context.Context, key string) error
	ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error)
}

type SessionStoreWithTimeout struct {
	SessionStore
	timeout time.Duration
}

func NewSessionStoreWithTimout(store SessionStore, timeout time.Duration) *SessionStoreWithTimeout {
	return &SessionStoreWithTimeout{
		SessionStore: store,
		timeout:      timeout,
	}
}

func (s *SessionStoreWithTimeout) getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), s.timeout)
}

func (s *SessionStoreWithTimeout) DefaultPutKey(key, value string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.SessionStore.PutKey(ctx, key, value)

}

func (s *SessionStoreWithTimeout) DefaultReadKey(key string) (string, bool, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.SessionStore.ReadKey(ctx, key)

}

func (s *SessionStoreWithTimeout) DefaultDeleteKey(key string) error {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.SessionStore.DeleteKey(ctx, key)
}

func (s *SessionStoreWithTimeout) DefaultReadPrefixKey(prefix string) (map[string]string, error) {
	ctx, cancel := s.getCtx()
	defer cancel()
	return s.SessionStore.ReadPrefixKey(ctx, prefix)
}
