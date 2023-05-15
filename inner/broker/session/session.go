package session

import (
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/pkg"
	"strings"
	"sync"
)

type MemorySession struct {
	mux             sync.RWMutex
	m               map[pkg.SessionKey]string
	subTopics       map[string]int32
	topicsMessageID map[string]string
}

func NewSession() *MemorySession {
	return &MemorySession{}
}

func (s *MemorySession) Set(key pkg.SessionKey, value string) {
	if s.m == nil {
		return
	}
	s.m[key] = value

}
func (s *MemorySession) Get(key pkg.SessionKey) string {
	if s.m == nil {
		return ""
	}
	return s.m[key]
}

func (s *MemorySession) Destroy() {
	s.m = nil
	return
}

func (s *MemorySession) GetWithPrefix(prefix pkg.SessionKey, keyWithPrefix bool) map[string]string {
	var (
		tmp = make(map[string]string)
	)
	for k, v := range s.m {
		if index := strings.Index(string(k), string(prefix)); index != -1 {
			if keyWithPrefix {
				tmp[string(k[index:])] = v
			} else {
				tmp[string(k)] = v
			}
		}
	}
	return tmp
}

func (s *MemorySession) OnceListenPublishEvent(clientID string, f func(topic, id string)) {
	event.ListenPublishToClientEvent(clientID, f)
}

func (s *MemorySession) ReadSubTopics() map[string]int32 {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var (
		m = make(map[string]int32)
	)
	for k, v := range s.subTopics {
		m[k] = v
	}
	return m
}

func (s *MemorySession) CreateSubTopics(topic string, qos int32) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.subTopics == nil {
		s.subTopics = make(map[string]int32)
	}
	s.subTopics[topic] = qos
}

func (s *MemorySession) DeleteSubTopics(topic string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.subTopics != nil {
		delete(s.subTopics, topic)
	}
	s.deleteTopicMessageID(topic)
}

func (s *MemorySession) CreateTopicMessageID(topic string, messageID string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.topicsMessageID == nil {
		s.topicsMessageID = make(map[string]string)
	}
	s.topicsMessageID[topic] = messageID
}

func (s *MemorySession) ReadTopicMessageID(topic string) string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if s.topicsMessageID == nil {
		return ""
	}
	return s.topicsMessageID[topic]
}

func (s *MemorySession) GetTopicsMessageID() map[string]string {
	var (
		m = make(map[string]string)
	)
	s.mux.RLock()
	defer s.mux.RUnlock()
	if s.topicsMessageID == nil {
		return m
	}
	for k, v := range s.topicsMessageID {
		m[k] = v
	}
	return m
}

func (s *MemorySession) deleteTopicMessageID(topic string) {
	if s.topicsMessageID != nil {
		delete(s.topicsMessageID, topic)
	}
}
