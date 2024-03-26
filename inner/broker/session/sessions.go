package session

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/broker/store"
	"time"
)

type Sessions struct {
	store *store.KeyValueStoreWithTimeout
}

// NewSessions returns a new session manager.
// The session manager is responsible for managing the sessions.
// Parameter store broker.HashStore is used to store the session data. if single node, you can use memory store.
func NewSessions(keyStore store.KeyStore) *Sessions {
	return &Sessions{store: store.NewKeyValueStoreWithTimout(keyStore, 3*time.Second)}
}

// ReadClientSession returns a client session.
func (s *Sessions) ReadClientSession(ctx context.Context, clientID string) (session.Session, bool) {
	if len(clientID) == 0 {
		return nil, false
	}
	m, err := s.store.DefaultReadPrefixKey(ctx, clientSessionPrefix(clientID))
	if err != nil {
		return nil, false
	}
	if len(m) > 0 {
		return newSession(ctx, clientID, s.store), true
	}
	return nil, false
}

// AddClientSession adds a client session.
func (s *Sessions) AddClientSession(ctx context.Context, key string, session session.Session) {
}

// NewClientSession returns a new client session.
func (s *Sessions) NewClientSession(ctx context.Context, key string) session.Session {
	return newSession(ctx, key, s.store)
}
