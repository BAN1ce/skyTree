package session

import "sync"

type Manager struct {
	mux      sync.RWMutex
	sessions map[string]*MemorySession
}

func NewSessionManager() *Manager {
	return &Manager{
		sessions: map[string]*MemorySession{},
	}
}

func (m *Manager) DeleteSession(key string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.sessions, key)
}

func (m *Manager) ReadSession(key string) (*MemorySession, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if session, ok := m.sessions[key]; ok {
		return session, true
	}
	return NewSession(), false
}

func (m *Manager) CreateSession(key string, session *MemorySession) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.sessions[key] = session
}
