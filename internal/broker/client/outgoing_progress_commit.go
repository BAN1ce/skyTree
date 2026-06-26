package client

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

// Periodic in-flight commit of downlink delivery progress.
//
// The QoS1 replay cursor and the QoS2/retained outgoing unfinished messages are otherwise only
// persisted to the session store at graceful disconnect (SaveOfflineState). On an unexpected
// crash that snapshot is missing, so on reconnect the broker could replay from a stale position
// and produce a large burst of redelivered messages. Committing the same progress periodically
// (every OutgoingCommitInterval, or every OutgoingCommitMaxAcks terminal ACKs) bounds the
// post-crash replay window to a single commit period.
//
// The commit covers both QoS classes: QoS1 via the replay cursor (position replay) and
// QoS2/retained via the unfinished entries (stage-accurate restore by PacketID + state). The
// QoS2/retained set is bounded by the downlink Receive Maximum window, so the committed payload
// stays small even at high throughput.

// recordOutgoingTerminalAcks counts downlink terminal ACKs (QoS1 PUBACK / QoS2 PUBCOMP) and
// triggers a count-based commit once OutgoingCommitMaxAcks is reached.
func (c *Client) recordOutgoingTerminalAcks(n int, cfg config.DeliveryRunner) {
	if c == nil || n <= 0 {
		return
	}
	pending := c.outgoingAcksSinceCommit.Add(int64(n))
	if cfg.OutgoingCommitMaxAcks > 0 && pending >= int64(cfg.OutgoingCommitMaxAcks) {
		c.maybeCommitOutgoingProgress()
	}
}

// maybeTimedOutgoingCommit triggers a time-based commit when at least one ACK has accumulated and
// OutgoingCommitInterval has elapsed since the last commit. It is called from the delivery runner
// loop, which is woken frequently enough (inflight tick, or a capped idle wait) to honor the interval.
func (c *Client) maybeTimedOutgoingCommit(cfg config.DeliveryRunner) {
	if c == nil || cfg.OutgoingCommitInterval <= 0 {
		return
	}
	if c.outgoingAcksSinceCommit.Load() <= 0 {
		return
	}
	last := c.lastOutgoingCommitUnixNano.Load()
	if last != 0 && time.Since(time.Unix(0, last)) < cfg.OutgoingCommitInterval {
		return
	}
	c.maybeCommitOutgoingProgress()
}

// hasPendingOutgoingCommit reports whether downlink progress has advanced since the last commit.
func (c *Client) hasPendingOutgoingCommit() bool {
	return c != nil && c.outgoingAcksSinceCommit.Load() > 0
}

// maybeCommitOutgoingProgress starts an asynchronous progress commit unless one is already running.
// The single-in-flight guard coalesces bursts of triggers into one raft write at a time.
func (c *Client) maybeCommitOutgoingProgress() {
	if c == nil {
		return
	}
	if !c.outgoingCommitInFlight.CompareAndSwap(false, true) {
		return
	}
	go c.commitOutgoingProgressToSession()
}

// commitOutgoingProgressToSession snapshots the current downlink progress and writes it to the
// session store. It runs in its own goroutine and never blocks the ACK path. On success it reduces
// the pending-ack counter by the amount observed at snapshot time, so ACKs that arrive during the
// write are preserved for the next commit.
func (c *Client) commitOutgoingProgressToSession() {
	defer c.outgoingCommitInFlight.Store(false)

	if c == nil || c.component == nil || c.component.sessionCenter == nil {
		return
	}
	clientID := c.getID()
	if clientID == "" {
		return
	}
	acks := c.outgoingAcksSinceCommit.Load()
	if acks <= 0 {
		return
	}

	unfinished := c.collectOutgoingUnfinishedMessages()
	if unfinished == nil {
		unfinished = make([]*proto_session.UnfinishedMessage, 0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.component.sessionCenter.CommitOutgoingProgress(ctx, &proto_session.CommitOutgoingProgressRequest{
		ClientID:             clientID,
		UnfinishedMessages:   unfinished,
		OutgoingReplayCursor: c.collectOutgoingReplayCursor(),
		OwnerToken:           c.getOwnerToken(),
		NowUnixNano:          time.Now().UnixNano(),
	}); err != nil {
		logger.Logger.Debug().
			Err(err).
			Str("client", c.metaString()).
			Msg("failed to commit outgoing delivery progress to session")
		return
	}

	c.lastOutgoingCommitUnixNano.Store(time.Now().UnixNano())
	c.outgoingAcksSinceCommit.Add(-acks)
}
