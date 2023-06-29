package session

import (
	"context"
	"sync"
)

type MemorySession struct {
	mux           sync.RWMutex
	subTopicsMeta *subTopicsMeta
}

func NewSession() *MemorySession {
	return &MemorySession{
		subTopicsMeta: newSubTopicsMeta(),
	}
}

func (s *MemorySession) Destroy() {
	// TODO: GC session
}

func (s *MemorySession) OnceListenTopicStoreEvent(ctx context.Context, clientID string, f func(topic, id string)) {
	// TODO: add store event listener
	panic("implement me")

}

func (s *MemorySession) ReadSubTopics() map[string]int32 {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.subTopicsMeta.ReadSubTopics()
}

func (s *MemorySession) CreateSubTopics(topic string, qos int32) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.subTopicsMeta.CreateSubTopics(topic, qos)
}

func (s *MemorySession) DeleteSubTopics(topic string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.subTopicsMeta.DeleteSubTopics(topic)
}

func (s *MemorySession) UpdateTopicLastAckedMessageID(topic string, messageID string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.subTopicsMeta.UpdateTopicLastAckedMessageID(topic, messageID)
}

func (s *MemorySession) ReadTopicLastAckedMessageID(topic string) (string, bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.subTopicsMeta.ReadTopicLastAckedMessageID(topic)
}

func (s *MemorySession) CreateTopicUnAckMessageID(topic string, messageID []string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.subTopicsMeta.CreateTopicUnAckMessageID(topic, messageID)
}

func (s *MemorySession) DeleteTopicUnAckMessageID(topic string, messageID string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.subTopicsMeta.DeleteTopicUnAckMessageID(topic, messageID)
}

func (s *MemorySession) ReadSubTopicsLastAckedMessageID() map[string]string {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.subTopicsMeta.ReadSubTopicsLastAckedMessageID()
}

func (s *MemorySession) ReleaseTopicSession(topic string) {
	// TODO implement me
	panic("implement me")
}

func (s *MemorySession) ReadTopicUnAckMessageID(topic string) []string {
	// TODO implement me
	return nil
}

func (s *MemorySession) CreateWill(topic string, qos int32, retain bool, payload []byte, properties map[string]string) {
	// TODO implement me

}
