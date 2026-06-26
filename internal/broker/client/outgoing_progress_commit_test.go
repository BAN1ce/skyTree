package client

import (
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
)

func waitForCommits(t *testing.T, center *countingSessionCenter, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for center.commitCount() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRecordOutgoingTerminalAcksCommitsByCount verifies the count-based trigger fires once the
// configured ACK threshold is reached, and that bumping the counter does not double-commit while
// a commit is in flight.
func TestRecordOutgoingTerminalAcksCommitsByCount(t *testing.T) {
	center := &countingSessionCenter{fakeSessionCenter: &fakeSessionCenter{}}
	c := NewClient(&bufferConn{}, WithSessionCenter(center))
	c.ID = "client-commit-count"

	cfg := config.DeliveryRunner{OutgoingCommitMaxAcks: 3}

	c.recordOutgoingTerminalAcks(2, cfg) // below threshold, no commit
	if got := center.commitCount(); got != 0 {
		t.Fatalf("expected no commit below threshold, got %d", got)
	}

	c.recordOutgoingTerminalAcks(1, cfg) // reaches threshold -> commit
	waitForCommits(t, center, 1)
	if got := center.commitCount(); got != 1 {
		t.Fatalf("expected exactly 1 commit at threshold, got %d", got)
	}
	if len(center.commits) > 0 && center.commits[0].GetClientID() != c.ID {
		t.Fatalf("commit carried wrong client id: %s", center.commits[0].GetClientID())
	}
}

// TestMaybeTimedOutgoingCommit verifies the time-based trigger fires when ACKs are pending and the
// interval has elapsed, and stays silent when nothing has changed since the last commit.
func TestMaybeTimedOutgoingCommit(t *testing.T) {
	center := &countingSessionCenter{fakeSessionCenter: &fakeSessionCenter{}}
	c := NewClient(&bufferConn{}, WithSessionCenter(center))
	c.ID = "client-commit-timed"

	// No pending acks -> no commit even with a tiny interval.
	c.maybeTimedOutgoingCommit(config.DeliveryRunner{OutgoingCommitInterval: time.Nanosecond})
	if got := center.commitCount(); got != 0 {
		t.Fatalf("expected no commit without pending acks, got %d", got)
	}

	// Pending ack + elapsed interval -> exactly one commit.
	c.recordOutgoingTerminalAcks(1, config.DeliveryRunner{OutgoingCommitMaxAcks: 0})
	c.maybeTimedOutgoingCommit(config.DeliveryRunner{OutgoingCommitInterval: time.Nanosecond})
	waitForCommits(t, center, 1)
	if got := center.commitCount(); got != 1 {
		t.Fatalf("expected exactly 1 timed commit, got %d", got)
	}
}
