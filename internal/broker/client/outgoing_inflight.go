package client

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Minimal outgoing broker-to-client inflight tracking for downlink QoS1/QoS2.
// We key by MQTT PacketID and advance the delivery cursor only after the QoS handshake completes.
type outgoingInflightState int

const (
	outInflightWaitingPubAck outgoingInflightState = iota + 1
	outInflightWaitingPubRec
	outInflightWaitingPubComp
)

type outgoingInflightEntry struct {
	PacketID uint16
	QoS      byte

	// Delivery task cursor to advance when this message is ACKed.
	TaskTS     time.Time
	TaskID     uuid.UUID
	Generation int64

	// For debugging/observability.
	MessageID uuid.UUID

	ShareGroup    string
	SharedTaskID  uuid.UUID
	Retained      bool
	PublishPacket []byte

	// FirstPubTime records when we first sent this downlink PUBLISH (best-effort).
	FirstPubTime time.Time

	// LastSendTime is the most recent time we sent/retransmitted the packet.
	LastSendTime time.Time

	// RetryCount counts how many retransmission attempts have been made (best-effort).
	RetryCount int

	Acked         bool
	FlowTokenHeld bool
	State         outgoingInflightState

	// PersistedInSession marks whether this entry currently exists in the session
	// store's unfinished list. Only entries restored from session on reconnect are
	// persisted; messages sent and acked within a single online session are never
	// written per-message (they are only snapshotted at disconnect). The ack path
	// uses this flag to skip the (no-op) session RemoveOutgoingUnfinished raft write
	// for entries that were never persisted.
	PersistedInSession bool
}

type outgoingInflightStore struct {
	mu sync.RWMutex
	m  map[uint16]*outgoingInflightEntry
}

func newOutgoingInflightStore() *outgoingInflightStore {
	return &outgoingInflightStore{m: make(map[uint16]*outgoingInflightEntry, 64)}
}

func (s *outgoingInflightStore) Put(e *outgoingInflightEntry) (replaced bool) {
	if s == nil || e == nil || e.PacketID == 0 {
		return false
	}
	s.mu.Lock()
	_, replaced = s.m[e.PacketID]
	s.m[e.PacketID] = e
	s.mu.Unlock()
	return replaced
}

func (s *outgoingInflightStore) Get(packetID uint16) (*outgoingInflightEntry, bool) {
	if s == nil || packetID == 0 {
		return nil, false
	}
	s.mu.RLock()
	e, ok := s.m[packetID]
	s.mu.RUnlock()
	return e, ok
}

func (s *outgoingInflightStore) UpdateState(packetID uint16, st outgoingInflightState) {
	if s == nil || packetID == 0 {
		return
	}
	s.mu.Lock()
	if e, ok := s.m[packetID]; ok && e != nil {
		e.State = st
	}
	s.mu.Unlock()
}

func (s *outgoingInflightStore) MarkAcked(packetID uint16, qos byte) (*outgoingInflightEntry, bool) {
	if s == nil || packetID == 0 {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[packetID]
	if !ok || e == nil || e.QoS != qos || e.Acked {
		return nil, false
	}
	e.Acked = true
	cp := *e
	return &cp, true
}

func (s *outgoingInflightStore) MarkFlowTokenHeld(packetID uint16) (*outgoingInflightEntry, bool) {
	if s == nil || packetID == 0 {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[packetID]
	if !ok || e == nil || e.FlowTokenHeld {
		return nil, false
	}
	e.FlowTokenHeld = true
	cp := *e
	return &cp, true
}

func (s *outgoingInflightStore) ClearFlowTokenHeld(packetID uint16) {
	if s == nil || packetID == 0 {
		return
	}
	s.mu.Lock()
	if e, ok := s.m[packetID]; ok && e != nil {
		e.FlowTokenHeld = false
	}
	s.mu.Unlock()
}

func (s *outgoingInflightStore) Delete(packetID uint16) (*outgoingInflightEntry, bool) {
	if s == nil || packetID == 0 {
		return nil, false
	}
	s.mu.Lock()
	e, ok := s.m[packetID]
	if ok {
		delete(s.m, packetID)
	}
	s.mu.Unlock()
	return e, ok
}

func (s *outgoingInflightStore) MarkSent(packetID uint16, now time.Time) (retryCount int, ok bool) {
	if s == nil || packetID == 0 {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[packetID]
	if !ok || e == nil {
		return 0, false
	}
	e.LastSendTime = now
	e.RetryCount++
	return e.RetryCount, true
}

func (s *outgoingInflightStore) PopAckedInOrder() []*outgoingInflightEntry {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.m) == 0 {
		return nil
	}
	entries := make([]*outgoingInflightEntry, 0, len(s.m))
	for _, e := range s.m {
		if e == nil {
			continue
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return outgoingInflightLess(entries[i], entries[j])
	})

	out := make([]*outgoingInflightEntry, 0, len(entries))
	for _, e := range entries {
		if !e.Acked {
			break
		}
		cp := *e
		out = append(out, &cp)
		delete(s.m, e.PacketID)
	}
	return out
}

// Len returns the current inflight size on a best-effort basis.
func (s *outgoingInflightStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	n := len(s.m)
	s.mu.RUnlock()
	return n
}

// All returns a snapshot copy of all inflight entries (best-effort).
func (s *outgoingInflightStore) All() []*outgoingInflightEntry {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.m) == 0 {
		return nil
	}
	out := make([]*outgoingInflightEntry, 0, len(s.m))
	for _, e := range s.m {
		if e == nil {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// First returns one inflight entry snapshot (best-effort).
func (s *outgoingInflightStore) First() (*outgoingInflightEntry, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.m {
		if e == nil {
			continue
		}
		cp := *e
		return &cp, true
	}
	return nil, false
}

func (s *outgoingInflightStore) FirstUnacked() (*outgoingInflightEntry, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var first *outgoingInflightEntry
	for _, e := range s.m {
		if e == nil || e.Acked {
			continue
		}
		if first == nil || outgoingInflightLess(e, first) {
			cp := *e
			first = &cp
		}
	}
	if first == nil {
		return nil, false
	}
	return first, true
}

func (s *outgoingInflightStore) LastTask() (*outgoingInflightEntry, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var last *outgoingInflightEntry
	for _, e := range s.m {
		if e == nil {
			continue
		}
		if last == nil || outgoingInflightLess(last, e) {
			cp := *e
			last = &cp
		}
	}
	if last == nil {
		return nil, false
	}
	return last, true
}

func outgoingInflightLess(a, b *outgoingInflightEntry) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}
	if !a.TaskTS.Equal(b.TaskTS) {
		return a.TaskTS.Before(b.TaskTS)
	}
	return a.TaskID.String() < b.TaskID.String()
}
