package session

import (
	"github.com/BAN1ce/skyTree/pkg"
	"time"
)

type Sessions struct {
	store *pkg.SessionStoreWithTimeout
}

func NewSessions(store pkg.SessionStore) *Sessions {
	return &Sessions{store: pkg.NewSessionStoreWithTimout(store, 3*time.Second)}

}

func (s *Sessions) ReadSession(key string) (pkg.Session, bool) {
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

func (s *Sessions) CreateSession(key string, session pkg.Session) {
	return
}

func (s *Sessions) NewSession(key string) pkg.Session {
	return newSession(key, s.store)
}
