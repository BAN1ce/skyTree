package session

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"time"
)

type Sessions struct {
	store *broker.SessionStoreWithTimeout
}

func NewSessions(store broker.SessionStore) *Sessions {
	return &Sessions{store: broker.NewSessionStoreWithTimout(store, 3*time.Second)}

}

func (s *Sessions) ReadSession(key string) (broker.Session, bool) {
	_, ok, err := s.store.DefaultReadKey(clientKey(key).String())
	if err != nil {
		return nil, false
	}
	if ok {
		return newSession(key, s.store), true
	}
	return nil, false
}

func (s *Sessions) DeleteSession(key string) {
	newSession(key, s.store).Release()
}

func (s *Sessions) CreateSession(key string, session broker.Session) {
	return
}

func (s *Sessions) NewSession(key string) broker.Session {
	return newSession(key, s.store)
}
