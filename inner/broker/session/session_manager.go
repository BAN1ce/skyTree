package session

import (
	"github.com/BAN1ce/skyTree/pkg"
	"sync"
)

type Manager struct {
	mux      sync.RWMutex
	sessions map[string]pkg.Session
}

func NewSessionManager() *Manager {
	return &Manager{
		sessions: map[string]pkg.Session{},
	}
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
	return NewSession(), false
}

func (m *Manager) CreateSession(key string, session pkg.Session) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.sessions[key] = session
}
