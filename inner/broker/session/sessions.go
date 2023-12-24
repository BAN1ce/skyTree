package session

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"time"
)

type Sessions struct {
	store *broker.KeyValueStoreWithTimeout
}

// NewSessions returns a new session manager.
// The session manager is responsible for managing the sessions.
// Parameter store broker.HashStore is used to store the session data. if single node, you can use memory store.
func NewSessions(store broker.KeyStore) *Sessions {
	return &Sessions{store: broker.NewKeyValueStoreWithTimout(store, 3*time.Second)}
}

// ReadClientSession returns a client session.
func (s *Sessions) ReadClientSession(clientID string) (session.Session, bool) {
	m, err := s.store.DefaultReadPrefixKey(clientSessionPrefix(clientID))
	if err != nil {
		return nil, false
	}
	if len(m) > 0 {
		return newSession(clientID, s.store), true
	}
	return nil, false
}

// AddClientSession adds a client session.
func (s *Sessions) AddClientSession(key string, session session.Session) {
}

// NewClientSession returns a new client session.
func (s *Sessions) NewClientSession(key string) session.Session {
	return newSession(key, s.store)
}
