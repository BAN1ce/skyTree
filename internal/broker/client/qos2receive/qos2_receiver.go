package qos2receive

import (
	"sync"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
)

// QoS2ReceiveStore is used to store the QoS2 packet that the client sends to the broker
type QoS2ReceiveStore struct {
	mux     sync.RWMutex
	waiting map[uint16]*brokerpublish.Message
	// firstSeen/lastSeen are best-effort timestamps used for TTL cleanup.
	firstSeen map[uint16]time.Time
	lastSeen  map[uint16]time.Time
}

const defaultQoS2WaitingTTL = 10 * time.Minute

func NewQoS2ReceiveStore() *QoS2ReceiveStore {
	return &QoS2ReceiveStore{
		waiting:   make(map[uint16]*brokerpublish.Message),
		firstSeen: make(map[uint16]time.Time),
		lastSeen:  make(map[uint16]time.Time),
	}
}

// CleanupExpired removes entries that haven't been touched within ttl.
// If ttl <= 0, CleanupExpired does nothing.
func (q *QoS2ReceiveStore) CleanupExpired(ttl time.Duration, now time.Time) (removed int) {
	if q == nil || ttl <= 0 {
		return 0
	}
	q.mux.Lock()
	defer q.mux.Unlock()

	for pid, last := range q.lastSeen {
		if now.Sub(last) <= ttl {
			continue
		}
		delete(q.waiting, pid)
		delete(q.firstSeen, pid)
		delete(q.lastSeen, pid)
		removed++
	}
	return removed
}

func (q *QoS2ReceiveStore) Store(message *brokerpublish.Message) (existed bool) {
	q.mux.Lock()
	defer q.mux.Unlock()

	// Validate input message.
	if message == nil {
		return false
	}

	if message.GetPublish() == nil {
		return false
	}

	var messageID uint16
	if publish := message.GetPublish(); publish != nil {
		messageID = publish.PacketID
	}

	// Best-effort cleanup to avoid PacketID being stuck forever.
	q.cleanupExpiredLocked(defaultQoS2WaitingTTL, time.Now())

	if _, ok := q.waiting[messageID]; ok {
		q.lastSeen[messageID] = time.Now()
		return true
	}

	q.waiting[messageID] = message
	now := time.Now()
	q.firstSeen[messageID] = now
	q.lastSeen[messageID] = now
	return false
}

func (q *QoS2ReceiveStore) Read(packetID uint16) (*brokerpublish.Message, bool) {
	q.mux.RLock()
	defer q.mux.RUnlock()

	if message, ok := q.waiting[packetID]; ok {
		return message, true
	}

	return nil, false
}

func (q *QoS2ReceiveStore) Delete(packetID uint16) (*brokerpublish.Message, bool) {
	q.mux.Lock()
	defer q.mux.Unlock()

	if message, ok := q.waiting[packetID]; !ok {
		return nil, false
	} else {
		delete(q.waiting, packetID)
		delete(q.firstSeen, packetID)
		delete(q.lastSeen, packetID)
		return message, true
	}
}

// GetAll returns all unfinished messages waiting for PUBREL
func (q *QoS2ReceiveStore) GetAll() []*brokerpublish.Message {
	q.mux.RLock()
	defer q.mux.RUnlock()

	if len(q.waiting) == 0 {
		return nil
	}

	messages := make([]*brokerpublish.Message, 0, len(q.waiting))
	for _, msg := range q.waiting {
		messages = append(messages, msg)
	}
	return messages
}

// Clear removes all messages from the store
func (q *QoS2ReceiveStore) Clear() {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.waiting = make(map[uint16]*brokerpublish.Message)
	q.firstSeen = make(map[uint16]time.Time)
	q.lastSeen = make(map[uint16]time.Time)
}

// Count returns the number of messages waiting for PUBREL
func (q *QoS2ReceiveStore) Count() int {
	q.CleanupExpired(defaultQoS2WaitingTTL, time.Now())

	q.mux.RLock()
	defer q.mux.RUnlock()
	return len(q.waiting)
}

func (q *QoS2ReceiveStore) cleanupExpiredLocked(ttl time.Duration, now time.Time) {
	if q == nil || ttl <= 0 {
		return
	}
	for pid, last := range q.lastSeen {
		if now.Sub(last) <= ttl {
			continue
		}
		delete(q.waiting, pid)
		delete(q.firstSeen, pid)
		delete(q.lastSeen, pid)
	}
}
