package broker

import (
	"github.com/eclipse/paho.golang/packets"
	"sync"
)

type subTree struct {
	mux     sync.RWMutex
	hashSub map[string]map[string]packets.SubOptions
}

func NewSubTree() *subTree {
	tmp := &subTree{hashSub: map[string]map[string]packets.SubOptions{}}
	return tmp
}

func (s *subTree) CreateSub(clientID string, topics map[string]packets.SubOptions) {
	s.mux.Lock()
	defer s.mux.Unlock()
	for topic, qos := range topics {
		if s.hashSub[topic] == nil {
			s.hashSub[topic] = map[string]packets.SubOptions{}
		}
		s.hashSub[topic][clientID] = qos
	}
}

func (s *subTree) DeleteSub(clientID string, topics []string) {
	// TODO implement me
	panic("implement me")
}

func (s *subTree) Match(topic string) (clientIDQos map[string]int32) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	clientIDQos = map[string]int32{}
	if clients := s.hashSub[topic]; clients != nil {
		for client, qos := range clients {
			clientIDQos[client] = int32(qos.QoS)
		}
	}
	return
}

func (s *subTree) HasSub(topic string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.hashSub[topic] != nil
}

func (s *subTree) DeleteClient(clientID string) {
	// TODO implement me
	panic("implement me")
}
