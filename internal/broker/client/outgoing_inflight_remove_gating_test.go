package client

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

// countingSessionCenter embeds the shared fake and records RemoveOutgoingUnfinished and
// CommitOutgoingProgress calls for assertions.
type countingSessionCenter struct {
	*fakeSessionCenter
	mu      sync.Mutex
	removed []string
	commits []*proto_session.CommitOutgoingProgressRequest
}

func (c *countingSessionCenter) RemoveOutgoingUnfinished(_ context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	c.mu.Lock()
	c.removed = append(c.removed, request.GetMessageID())
	c.mu.Unlock()
	return nil
}

func (c *countingSessionCenter) CommitOutgoingProgress(_ context.Context, request *proto_session.CommitOutgoingProgressRequest) error {
	c.mu.Lock()
	c.commits = append(c.commits, request)
	c.mu.Unlock()
	return nil
}

func (c *countingSessionCenter) commitCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.commits)
}

func (c *countingSessionCenter) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.removed...)
}

// TestAdvanceAckedSkipsSessionRemoveForNonPersisted verifies that, with the default
// SkipNonPersistedUnfinishedRemove behavior, the per-ack session RemoveOutgoingUnfinished
// raft write is issued only for entries that were restored from the session store
// (PersistedInSession=true) and skipped for messages first sent within this online session.
func TestAdvanceAckedSkipsSessionRemoveForNonPersisted(t *testing.T) {
	counting := &countingSessionCenter{fakeSessionCenter: &fakeSessionCenter{}}
	c := NewClient(&bufferConn{}, WithSessionCenter(counting))
	c.ID = "client-skip-gating"

	persistedID := uuid.New() // restored from a previous session -> must be removed on ack
	freshID := uuid.New()     // sent and acked online -> never persisted -> remove skipped

	putAcked := func(packetID uint16, msgID uuid.UUID, persisted bool) {
		c.outgoingInflight.Put(&outgoingInflightEntry{
			PacketID:           packetID,
			QoS:                2,
			Retained:           true,
			MessageID:          msgID,
			TaskID:             msgID,
			State:              outInflightWaitingPubComp,
			PersistedInSession: persisted,
		})
		if _, ok := c.outgoingInflight.MarkAcked(packetID, 2); !ok {
			t.Fatalf("failed to mark packet %d acked", packetID)
		}
	}
	putAcked(1, persistedID, true)
	putAcked(2, freshID, false)

	NewClientHandler(c).advanceAckedOutgoingInflight(context.Background())

	// removeOutgoingUnfinishedFromSession runs in goroutines; wait for the persisted one,
	// then allow extra time to confirm the non-persisted one never fires.
	deadline := time.Now().Add(time.Second)
	for len(counting.snapshot()) < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	removed := counting.snapshot()
	if len(removed) != 1 {
		t.Fatalf("expected exactly 1 RemoveOutgoingUnfinished call (persisted only), got %d: %v", len(removed), removed)
	}
	if removed[0] != persistedID.String() {
		t.Fatalf("expected remove for persisted message %s, got %s", persistedID, removed[0])
	}
}
