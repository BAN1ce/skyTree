package clientalive

import (
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/pkg/indexedheap"
)

type aliveValue struct {
	clientID   string
	ownerToken string
	lastAlive  time.Time
	expireAt   time.Time
}

type ExpiredClient struct {
	ClientID   string
	OwnerToken string
}

// Tracker maintains node-local client alive state.
// It is runtime-only state: it is neither persisted nor replicated across nodes.
type Tracker struct {
	mu    sync.Mutex
	items *indexedheap.Queue[string, aliveValue]
}

func NewTracker() *Tracker {
	t := &Tracker{
		items: indexedheap.New[string, aliveValue](),
	}
	return t
}

func (t *Tracker) Update(clientID, ownerToken string, at time.Time, keepAlive time.Duration) {
	if t == nil || clientID == "" || ownerToken == "" || at.IsZero() || keepAlive <= 0 {
		return
	}

	expireAt := at.Add(keepAlive + keepAlive/2)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.items.Upsert(clientID, expireAt.UnixNano(), aliveValue{
		clientID:   clientID,
		ownerToken: ownerToken,
		lastAlive:  at,
		expireAt:   expireAt,
	})
}

func (t *Tracker) DeleteIfOwner(clientID, ownerToken string) bool {
	if t == nil || clientID == "" || ownerToken == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	item, ok := t.items.Get(clientID)
	if !ok || item.ownerToken != ownerToken {
		return false
	}
	return t.items.Remove(clientID)
}

func (t *Tracker) ScanExpired(now time.Time) []ExpiredClient {
	if t == nil || now.IsZero() {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	due := t.items.ValuesAtOrBefore(now.UnixNano())
	expired := make([]ExpiredClient, 0, len(due))
	for _, item := range due {
		if !t.items.Remove(item.clientID) {
			continue
		}
		expired = append(expired, ExpiredClient{
			ClientID:   item.clientID,
			OwnerToken: item.ownerToken,
		})
	}
	return expired
}

func (t *Tracker) LastAlive(clientID string) (time.Time, bool) {
	if t == nil || clientID == "" {
		return time.Time{}, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	item, ok := t.items.Get(clientID)
	if !ok {
		return time.Time{}, false
	}
	return item.lastAlive, true
}

func (t *Tracker) ExpireAt(clientID string) (time.Time, bool) {
	if t == nil || clientID == "" {
		return time.Time{}, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	item, ok := t.items.Get(clientID)
	if !ok {
		return time.Time{}, false
	}
	return item.expireAt, true
}
