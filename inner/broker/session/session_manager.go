package session

import (
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/db"
	"sync"
)

const (
	TypeLocal = iota
	TypeRedis
	TypeMemory
)

type Option func(*Manager)

func WithType(t int) Option {
	return func(m *Manager) {
		m.sessionType = t
	}
}

type Manager struct {
	mux         sync.RWMutex
	sessions    map[string]pkg.Session
	sessionType int
}

func NewSessionManager(options ...Option) *Manager {
	m := &Manager{
		sessions: map[string]pkg.Session{},
	}
	for _, option := range options {
		option(m)
	}
	return m
}

func (m *Manager) DeleteSession(key string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.sessions, key)
}

func (m *Manager) ReadSession(key string) (pkg.Session, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if session, ok := m.sessions[key]; ok {
		return session, true
	}
	return m.NewSession(key), false
}

func (m *Manager) CreateSession(key string, session pkg.Session) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.sessions[key] = session
}

func (m *Manager) NewSession(clientID string) pkg.Session {
	f := Factory(m.sessionType)
	return f(clientID)
}

func Factory(sessionType int) func(key string) pkg.Session {
	switch sessionType {
	case TypeLocal:
		return func(key string) pkg.Session {
			return NewLocalSession(db.GetNutsDB(), key)
		}
	case TypeRedis:
		panic("not implement")

	case TypeMemory:
		return func(key string) pkg.Session {
			return NewSession(key)
		}

	}
	return nil
}
