package client

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetpool"
	"github.com/google/uuid"
)

// ClientDeliveryRunner drains the delivery_task queue for an online client and writes PUBLISH packets to the socket.
// This is the runtime counterpart of the client-centric delivery pipeline.

type outgoingReplayCursor struct {
	TaskTS     time.Time
	TaskID     uuid.UUID
	Generation int64
}

// StartClientDeliveryRunner registers the per-client delivery listener and starts the
// background runner goroutine (idempotent). The runner drains the client's delivery
// task queue and writes PUBLISH packets to the socket.
func (c *Client) StartClientDeliveryRunner() {
	if c == nil {
		return
	}
	// Avoid calling exported Client getters here (GetID/MetaString/Close/etc.) because they may re-lock c.mux and deadlock.
	clientID := c.getID()
	c.deliveryListenerOnce.Do(func() {
		c.deliveryWakeCh = make(chan struct{}, 1)
		if c.component != nil && c.component.clientDeliveryEvent != nil && clientID != "" {
			if id, _, err := c.component.clientDeliveryEvent.AddListener(c.ctx, clientID, c.onClientDeliveryNotify); err == nil {
				c.deliveryListenerID = id
			} else {
				logger.Logger.Warn().Err(err).Str("client", c.metaString()).Msg("register client delivery listener failed")
			}
		}
	})
	// Always start the client-centric delivery runner; legacy topic delivery is intentionally disabled.
	c.deliveryRunnerOnce.Do(func() {
		go c.runClientDeliveryRunner()
	})
}

// WakeDeliveryRunner nudges the runner to re-check the delivery store immediately.
// The wake channel is buffered with size 1, so signals coalesce and never block.
func (c *Client) WakeDeliveryRunner() {
	if c == nil || c.deliveryWakeCh == nil {
		return
	}
	select {
	case c.deliveryWakeCh <- struct{}{}:
	default:
	}
}

// onClientDeliveryNotify handles per-client local delivery notifications.
// - KindWake: wake runner to re-check store (with backoff).
// - KindQoS0Direct: write publish to socket directly (no persistence).
func (c *Client) onClientDeliveryNotify(n *deliveryevent.Notify) {
	if !c.acceptsClientDeliveryNotify(n) {
		return
	}
	c.handleClientDeliveryNotify(n)
}

func (c *Client) acceptsClientDeliveryNotify(n *deliveryevent.Notify) bool {
	if c == nil || c.component == nil {
		return false
	}
	// Fencing: ignore stale client instances for the same ClientID.
	if c.component.clientManager != nil {
		if cur, ok := c.component.clientManager.ReadClient(c.ID); ok && cur != nil && cur != c {
			return false
		}
	}
	if c.ctx.Err() != nil {
		return false
	}
	return n != nil
}

// handleClientDeliveryNotify dispatches an accepted delivery notification: a wake
// signal re-checks the store, while a QoS0 direct notification is written immediately.
func (c *Client) handleClientDeliveryNotify(n *deliveryevent.Notify) {
	switch n.Kind {
	case deliveryevent.KindWake:
		c.WakeDeliveryRunner()
	case deliveryevent.KindQoS0Direct:
		c.handleQoS0DirectNotify(n)
	}
}

// handleQoS0DirectNotify writes a QoS0 PUBLISH straight to the socket without
// persistence or inflight tracking, since QoS0 expects no acknowledgement.
func (c *Client) handleQoS0DirectNotify(n *deliveryevent.Notify) {
	if !c.canDeliverQoS0DirectNotify(n) {
		return
	}
	newPublishPacket, release := c.prepareQoS0DirectPublish(n)
	if newPublishPacket == nil {
		return
	}
	defer release()

	_ = c.Write(&clientcap.WritePacket{Packet: newPublishPacket, FullTopic: n.Message.GetPublish().Topic})
}

// canDeliverQoS0DirectNotify reports whether a QoS0 direct notification may be sent,
// applying message-expiry, payload-format validation, and the No Local rule.
func (c *Client) canDeliverQoS0DirectNotify(n *deliveryevent.Notify) bool {
	if n.Message == nil || n.Message.GetPublish() == nil {
		return false
	}
	publish := n.Message.GetPublish()
	if !applyMessageExpiryForDelivery(publish, n.Message, time.Now()) {
		return false
	}
	if outboundPayloadFormatInvalid(publish) {
		logger.Logger.Warn().Str("client", c.MetaString()).Str("topic", publish.Topic).
			Msg("drop QoS0 direct: payload format indicator=1 but payload is not valid UTF-8")
		return false
	}
	// NL (No Local): do not deliver to the same client that published the message.
	return !n.NoLocal || n.Message.SendClientID == "" || n.Message.SendClientID != c.getID()
}

// prepareQoS0DirectPublish builds a pooled PUBLISH copy for a QoS0 direct send and
// returns it together with a release func to return the packet to the pool.
func (c *Client) prepareQoS0DirectPublish(n *deliveryevent.Notify) (*packets.ControlPacket, func()) {
	originalCP := n.Message.GetControlPacket()
	if originalCP == nil {
		return nil, nil
	}

	newPublishPacket := packetpool.PublishPool.Get()
	packetpool.CopyPublish(newPublishPacket, originalCP)
	if publishContent, ok := newPublishPacket.Content.(*packets.Publish); ok {
		applyQoS0DirectPublishOptions(publishContent, n)
	}
	return newPublishPacket, func() {
		packetpool.PublishPool.Put(newPublishPacket)
	}
}

// applyQoS0DirectPublishOptions normalizes a QoS0 outbound PUBLISH: forces QoS 0 and a
// zero PacketID, honors Retain As Published, and attaches matched subscription identifiers.
func applyQoS0DirectPublishOptions(publishContent *packets.Publish, n *deliveryevent.Notify) {
	publishContent.QoS = 0
	publishContent.PacketID = 0
	if !n.RAP {
		publishContent.Retain = false
	}
	var subIDs []int32
	if n.SubscriptionIDsJSON != "" {
		_ = json.Unmarshal([]byte(n.SubscriptionIDsJSON), &subIDs)
	}
	attachDeliverySubscriptionIDs(publishContent, subIDs)
}

// runClientDeliveryRunner is the main event-driven delivery loop for an online client.
// It loads the persisted cursor, then repeatedly waits for wake/tick, reads a batch of
// delivery tasks from the cursor position, and sends them in order while honoring the
// downlink Receive Maximum window and inflight retransmission.
func (c *Client) runClientDeliveryRunner() {
	// Best-effort: if components are missing, just exit.
	if c.component == nil || c.component.deliveryCursorStore == nil {
		return
	}
	cursorStore := c.component.deliveryCursorStore

	// Event-driven loop. Avoid per-client polling at scale.
	const (
		batchSize    = 200
		errorBackoff = 1 * time.Second
	)

	runnerCfg := c.getDeliveryRunnerConfig()
	lastTS, lastTask := c.loadClientDeliveryCursor(cursorStore)
	c.alignDeliveryCursorBeforeRunner(cursorStore, batchSize, errorBackoff, &lastTS, &lastTask)

	for {
		if c.ctx.Err() != nil {
			return
		}
		// Time-based commit of downlink progress to the session store, bounding post-crash replay.
		c.maybeTimedOutgoingCommit(runnerCfg)
		switch c.prepareDeliveryRunnerProbe(runnerCfg) {
		case deliveryRunnerExit:
			return
		case deliveryRunnerContinue:
			continue
		}
		// While progress is uncommitted, cap the idle wait so the loop wakes in time to honor
		// OutgoingCommitInterval even when the client is idle (all messages sent, none new).
		probeCfg := c.cappedProbeConfig(runnerCfg)
		tasks, exit := c.readDeliveryTasksForRunner(cursorStore, probeCfg, batchSize, lastTS, lastTask)
		if exit {
			return
		}
		if len(tasks) == 0 {
			continue
		}
		c.deliverClientDeliveryTasks(cursorStore, tasks, errorBackoff, &lastTS, &lastTask)
	}
}

// cappedProbeConfig shortens the idle WakeMaxWait to OutgoingCommitInterval when an outgoing
// progress commit is pending, so an idle client still commits within the configured interval.
func (c *Client) cappedProbeConfig(runnerCfg config.DeliveryRunner) config.DeliveryRunner {
	if runnerCfg.OutgoingCommitInterval > 0 &&
		runnerCfg.OutgoingCommitInterval < runnerCfg.WakeMaxWait &&
		c.hasPendingOutgoingCommit() {
		runnerCfg.WakeMaxWait = runnerCfg.OutgoingCommitInterval
	}
	return runnerCfg
}

type deliveryRunnerDecision byte

const (
	deliveryRunnerProbe deliveryRunnerDecision = iota
	deliveryRunnerContinue
	deliveryRunnerExit
)

func (c *Client) alignDeliveryCursorBeforeRunner(
	cursorStore delivery.CursorStore,
	batchSize int,
	errorBackoff time.Duration,
	lastTS *time.Time,
	lastTask *uuid.UUID,
) {
	// If a downlink inflight message was restored from session state, align the in-memory cursor
	// to the newest restored delivery task to avoid resending it before ACK arrives.
	if c.outgoingReplayCursor == nil && c.outgoingInflight != nil && c.outgoingInflight.Len() > 0 {
		if e, ok := c.outgoingInflight.LastTask(); ok && e != nil && !e.Retained {
			*lastTS = e.TaskTS
			*lastTask = e.TaskID
		}
		return
	}

	// First connection probe. Do not retry; if empty, wait for wake.
	tasks, err := cursorStore.ReadTasks(c.ctx, c.GetID(), *lastTS, *lastTask, batchSize)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Msg("initial read delivery tasks failed")
		return
	}
	if len(tasks) > 0 {
		c.deliverClientDeliveryTasks(cursorStore, tasks, errorBackoff, lastTS, lastTask)
	}
}

// prepareDeliveryRunnerProbe runs before each store probe. It drives retransmission of
// restored/unacked inflight messages and blocks new sends while the downlink Receive
// Maximum window is full, returning whether the loop should probe, continue, or exit.
func (c *Client) prepareDeliveryRunnerProbe(runnerCfg config.DeliveryRunner) deliveryRunnerDecision {
	if c.hasDeferredRestoredOutgoingInflight() {
		c.maybeRetransmitOutgoingInflight(runnerCfg)
		if c.hasDeferredRestoredOutgoingInflight() {
			if c.waitInflightTick(runnerCfg) {
				return deliveryRunnerContinue
			}
			return deliveryRunnerExit
		}
	}

	// If the negotiated downlink Receive Maximum window is full, wait for an ACK.
	if !c.canSendMoreDownlink() {
		c.maybeRetransmitOutgoingInflight(runnerCfg)
		if !c.waitInflightTick(runnerCfg) {
			return deliveryRunnerExit
		}
		if !c.canSendMoreDownlink() {
			return deliveryRunnerContinue
		}
	}
	return deliveryRunnerProbe
}

// waitInflightTick waits for a wake signal or the inflight safety tick while the client
// has unacked inflight messages. It returns false only when the client context is done.
func (c *Client) waitInflightTick(runnerCfg config.DeliveryRunner) bool {
	if c.deliveryWakeCh == nil {
		time.Sleep(runnerCfg.InflightWaitTick)
		return true
	}
	select {
	case <-c.ctx.Done():
		return false
	case <-c.deliveryWakeCh:
	case <-time.After(runnerCfg.InflightWaitTick):
	}
	return true
}

// readDeliveryTasksForRunner waits for a wake or fallback timeout, then reads one batch of
// delivery tasks starting after (lastTS, lastTask). On a wake-triggered read failure it
// retries with backoff to tolerate async write visibility. The bool return signals ctx exit.
func (c *Client) readDeliveryTasksForRunner(
	cursorStore delivery.CursorStore,
	runnerCfg config.DeliveryRunner,
	batchSize int,
	lastTS time.Time,
	lastTask uuid.UUID,
) ([]*store.DeliveryTask, bool) {
	// Wait for wake or fallback timeout, then probe once.
	woke, exit := c.waitForDeliveryWake(runnerCfg.WakeMaxWait)
	if exit {
		return nil, true
	}

	tasks, err := cursorStore.ReadTasks(c.ctx, c.GetID(), lastTS, lastTask, batchSize)
	if err == nil {
		return tasks, false
	}
	// Only retry reads after a wake-triggered failure to handle async write visibility.
	if woke {
		tasks, _ = c.recheckClientDeliveryTasksAfterWakeReadError(cursorStore, lastTS, lastTask, batchSize, runnerCfg)
		return tasks, false
	}
	logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Msg("fallback read delivery tasks failed")
	return nil, false
}

// maybeRetransmitOutgoingInflight retransmits the oldest unacked downlink inflight message
// when its retransmit interval has elapsed. It disconnects the client when the message
// exceeds the max age or retry budget, and re-acquires a flow token before resending.
func (c *Client) maybeRetransmitOutgoingInflight(runnerCfg config.DeliveryRunner) {
	e, ok := c.firstRetransmittableInflight()
	if !ok {
		return
	}
	now := time.Now()
	if c.disconnectForExpiredRetransmit(e, runnerCfg, now) || shouldWaitForRetransmitInterval(e, runnerCfg, now) {
		return
	}
	e, tokenAcquired, ok := c.acquireRetransmitFlowToken(e)
	if !ok {
		return
	}

	if err := c.retransmitOutgoingInflightEntry(e); err != nil {
		c.handleOutgoingRetransmitError(e, tokenAcquired, err)
		return
	}
	if _, ok := c.outgoingInflight.MarkSent(e.PacketID, now); ok {
		metric.RecordDeliverySendAttempt(deliveryPathFromInflight(e), int(e.QoS), "retransmit")
	}
}

func (c *Client) firstRetransmittableInflight() (*outgoingInflightEntry, bool) {
	if c == nil || c.outgoingInflight == nil {
		return nil, false
	}
	e, ok := c.outgoingInflight.FirstUnacked()
	return e, ok && e != nil && e.PacketID != 0
}

func (c *Client) disconnectForExpiredRetransmit(
	e *outgoingInflightEntry,
	runnerCfg config.DeliveryRunner,
	now time.Time,
) bool {
	// 超时 / 超重试上限 => MQTT5 §4.13 用 0x97 Quota Exceeded 通知客户端，
	// 保留 session 中的未完成 inflight，等待重连后恢复。
	if runnerCfg.InflightMaxAge > 0 && !e.FirstPubTime.IsZero() && now.Sub(e.FirstPubTime) > runnerCfg.InflightMaxAge {
		_ = c.write(&clientcap.WritePacket{Packet: disconnectForQuotaExceeded("downlink inflight expired without ACK")})
		_ = c.close()
		return true
	}
	if runnerCfg.InflightMaxRetries >= 0 && e.RetryCount >= runnerCfg.InflightMaxRetries {
		_ = c.write(&clientcap.WritePacket{Packet: disconnectForQuotaExceeded("downlink inflight retransmission limit exceeded")})
		_ = c.close()
		return true
	}
	return false
}

func shouldWaitForRetransmitInterval(e *outgoingInflightEntry, runnerCfg config.DeliveryRunner, now time.Time) bool {
	return runnerCfg.InflightRetransmitInterval > 0 &&
		!e.LastSendTime.IsZero() &&
		now.Sub(e.LastSendTime) < runnerCfg.InflightRetransmitInterval
}

func (c *Client) acquireRetransmitFlowToken(e *outgoingInflightEntry) (*outgoingInflightEntry, bool, bool) {
	if e.FlowTokenHeld || c.publishBucket == nil {
		return e, false, true
	}
	if !c.publishBucket.TryGetToken() {
		return e, false, false
	}
	updated, ok := c.outgoingInflight.MarkFlowTokenHeld(e.PacketID)
	if !ok {
		c.publishBucket.PutToken()
		return e, false, false
	}
	return updated, true, true
}

// retransmitOutgoingInflightEntry resends the packet appropriate to the entry's QoS stage:
// PUBREL while waiting for PUBCOMP, otherwise the original PUBLISH with DUP=1 and the same
// PacketID (reloading the payload from the delivery task for non-retained messages).
func (c *Client) retransmitOutgoingInflightEntry(e *outgoingInflightEntry) error {
	switch e.State {
	case outInflightWaitingPubComp:
		pubRelCP := packets.NewControlPacket(packets.PUBREL)
		pubRelCP.Content = &packets.Pubrel{PacketID: e.PacketID, ReasonCode: packets.PubrelSuccess}
		return c.Write(&clientcap.WritePacket{Packet: pubRelCP})
	default:
		if e.Retained && len(e.PublishPacket) > 0 {
			return c.retransmitRetainedPublishFromEntry(e)
		}
		// WAITING_PUBACK / WAITING_PUBREC: retransmit PUBLISH with DUP=1 and the same PacketID.
		t, err := c.findDeliveryTaskByMessageID(c.ctx, e.MessageID)
		if err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.metaString()).Uint16("packetID", e.PacketID).Msg("downlink inflight retransmit: delivery task not found")
			return err
		}
		if err := c.retransmitPublishFromTask(c.ctx, t, e.QoS, e.PacketID); err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.metaString()).Uint16("packetID", e.PacketID).Msg("downlink inflight retransmit: publish failed")
			return err
		}
		return nil
	}
}

func (c *Client) handleOutgoingRetransmitError(e *outgoingInflightEntry, tokenAcquired bool, err error) {
	// MQTT5 §3.1.2.11.4：重传时若新协商的 Maximum Packet Size 已经容不下原报文，
	// 等同于"已成功投递（消息被丢弃）"——清掉 inflight、释放 token，让 cursor 不再受阻。
	if IsOversizedOutboundPublish(err) {
		if _, ok := c.outgoingInflight.Delete(e.PacketID); ok && c.publishBucket != nil {
			c.publishBucket.PutToken()
		} else if tokenAcquired && c.publishBucket != nil {
			c.publishBucket.PutToken()
		}
		return
	}
	if tokenAcquired && c.publishBucket != nil {
		c.outgoingInflight.ClearFlowTokenHeld(e.PacketID)
		c.publishBucket.PutToken()
	}
}

// getDeliveryRunnerConfig returns delivery runner config with safe defaults.
// Some tests may not initialize global config; avoid panics by falling back to DefaultDeliveryRunner.
func (c *Client) getDeliveryRunnerConfig() config.DeliveryRunner {
	cfg := config.DefaultDeliveryRunner()
	cfg = c.deliveryRuntimeConfig()
	// Basic sanitization.
	if cfg.WakeMaxWait <= 0 {
		cfg.WakeMaxWait = 3 * time.Minute
	}
	if cfg.InflightWaitTick <= 0 {
		cfg.InflightWaitTick = 1 * time.Second
	}
	if cfg.InflightRetransmitInterval <= 0 {
		cfg.InflightRetransmitInterval = 5 * time.Second
	}
	if cfg.InflightMaxRetries < 0 {
		cfg.InflightMaxRetries = 0
	}
	if cfg.InflightMaxAge <= 0 {
		cfg.InflightMaxAge = 2 * time.Minute
	}
	if cfg.WakeReadRetryMaxAttempts < 0 {
		cfg.WakeReadRetryMaxAttempts = 0
	}
	if cfg.WakeReadRetryBackoffInitial <= 0 {
		cfg.WakeReadRetryBackoffInitial = 50 * time.Millisecond
	}
	if cfg.WakeReadRetryBackoffMax <= 0 {
		cfg.WakeReadRetryBackoffMax = 1 * time.Second
	}
	// Negative values are meaningless; treat them as 0 (disabled) for the progress-commit knobs.
	if cfg.OutgoingCommitInterval < 0 {
		cfg.OutgoingCommitInterval = 0
	}
	if cfg.OutgoingCommitMaxAcks < 0 {
		cfg.OutgoingCommitMaxAcks = 0
	}
	return cfg
}

// canSendMoreDownlink reports whether the negotiated downlink Receive Maximum window has
// room for another QoS1/QoS2 send (or, without a flow bucket, whether nothing is inflight).
func (c *Client) canSendMoreDownlink() bool {
	if c == nil {
		return false
	}
	if c.publishBucket != nil {
		return c.publishBucket.RemainingToken() > 0
	}
	return c.outgoingInflight == nil || c.outgoingInflight.Len() == 0
}

func (c *Client) hasDeferredRestoredOutgoingInflight() bool {
	if c == nil || c.outgoingInflight == nil {
		return false
	}
	e, ok := c.outgoingInflight.FirstUnacked()
	return ok && e != nil && !e.FlowTokenHeld
}

// loadClientDeliveryCursor loads the persisted delivery cursor if present.
func (c *Client) loadClientDeliveryCursor(cursorStore delivery.CursorStore) (time.Time, uuid.UUID) {
	lastTS := time.UnixMicro(0)
	lastTask := uuid.Nil

	if c == nil || cursorStore == nil {
		return lastTS, lastTask
	}
	if cur, err := cursorStore.ReadCursor(c.ctx, c.GetID()); err == nil && cur != nil {
		lastTS = cur.LastTS
		lastTask = cur.LastTaskID
	}
	if replay := c.outgoingReplayCursor; replay != nil {
		replayStart := replay.TaskTS.Add(-time.Microsecond)
		if replayStart.Before(time.UnixMicro(0)) {
			replayStart = time.UnixMicro(0)
		}
		lastTS = replayStart
		lastTask = uuid.Nil
		logger.Logger.Debug().
			Str("client", c.MetaString()).
			Str("task_id", replay.TaskID.String()).
			Int64("generation", replay.Generation).
			Msg("loaded qos1 replay cursor")
	}
	return lastTS, lastTask
}

// readClientDeliveryTasks fetches tasks, and if empty, waits for wake or tick before re-checking.
// Returns (tasks, exit) where exit indicates ctx cancellation.
// waitForDeliveryWake waits for a wake signal or fallback timeout.
// It returns (woke, exit) where exit indicates ctx cancellation.
func (c *Client) waitForDeliveryWake(maxWait time.Duration) (bool, bool) {
	if maxWait <= 0 {
		maxWait = 3 * time.Minute
	}
	// No wake channel => only fallback sleep.
	if c.deliveryWakeCh == nil {
		select {
		case <-c.ctx.Done():
			return false, true
		case <-time.After(maxWait):
			return false, false
		}
	}
	select {
	case <-c.ctx.Done():
		return false, true
	case <-c.deliveryWakeCh:
		return true, false
	case <-time.After(maxWait):
		return false, false
	}
}

// recheckClientDeliveryTasksAfterWakeReadError retries only after a wake-triggered ReadTasks failure.
// It stops retrying once ReadTasks succeeds (even if it returns empty) to avoid DB pressure.
func (c *Client) recheckClientDeliveryTasksAfterWakeReadError(
	cursorStore delivery.CursorStore,
	lastTS time.Time,
	lastTask uuid.UUID,
	batchSize int,
	cfg config.DeliveryRunner,
) ([]*store.DeliveryTask, bool) {
	maxAttempts := cfg.WakeReadRetryMaxAttempts
	if maxAttempts <= 0 {
		return nil, false
	}
	backoff := cfg.WakeReadRetryBackoffInitial
	if backoff <= 0 {
		backoff = 50 * time.Millisecond
	}
	maxBackoff := cfg.WakeReadRetryBackoffMax
	if maxBackoff <= 0 {
		maxBackoff = 1 * time.Second
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if c.ctx.Err() != nil {
			return nil, true
		}
		select {
		case <-c.ctx.Done():
			return nil, true
		case <-time.After(backoff):
		}

		tasks, err := cursorStore.ReadTasks(c.ctx, c.GetID(), lastTS, lastTask, batchSize)
		if err == nil {
			// Success (may be empty): stop retrying immediately.
			return tasks, false
		}

		// Increase backoff up to maxBackoff.
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return nil, false
}

// deliverClientDeliveryTasks delivers tasks in order and advances the cursor after each successful write.
// It stops the batch on first failure (to retry later).
func (c *Client) deliverClientDeliveryTasks(
	cursorStore delivery.CursorStore,
	tasks []*store.DeliveryTask,
	errorBackoff time.Duration,
	lastTS *time.Time,
	lastTask *uuid.UUID,
) {
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if c.ctx.Err() != nil {
			return
		}
		if !c.canSendMoreDownlink() {
			return
		}
		if err := c.deliverSingleClientDeliveryTask(cursorStore, t, lastTS, lastTask); err != nil {
			// Let outer loop exit on ctx cancel; otherwise backoff and retry later.
			time.Sleep(errorBackoff)
			break
		}

		if !c.canSendMoreDownlink() {
			return
		}
	}
}

// deliverSingleClientDeliveryTask loads message payload, decodes it, writes PUBLISH, and advances the cursor.
func (c *Client) deliverSingleClientDeliveryTask(
	cursorStore delivery.CursorStore,
	t *store.DeliveryTask,
	lastTS *time.Time,
	lastTask *uuid.UUID,
) error {
	msg, handled, err := c.loadClientDeliveryMessage(cursorStore, t, lastTS, lastTask)
	if handled || err != nil {
		return err
	}

	// NL: if configured and publisher == receiver, skip delivery but still advance cursor to avoid stalling.
	if t.NoLocal && msg.SendClientID != "" && msg.SendClientID == c.getID() {
		return c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}

	deliveryPublish, err := c.prepareDeliveryPublishCopy(msg)
	if err != nil {
		return err
	}
	defer deliveryPublish.release()

	if handled, err := c.applyDeliveryPublishOptions(cursorStore, t, lastTS, lastTask, msg, deliveryPublish.publish); handled || err != nil {
		return err
	}

	qos := deliveryQoS(t.DeliveryQoS)
	inflight, err := c.prepareDeliveryQoSAndInflight(t, deliveryPublish.publish, qos)
	if err != nil {
		return err
	}
	if err := c.Write(&clientcap.WritePacket{
		Packet:    deliveryPublish.packet,
		FullTopic: deliveryPublish.fullTopic,
	}); err != nil {
		return c.handleDeliveryWriteError(cursorStore, t, lastTS, lastTask, deliveryPublish, inflight, err)
	}
	observeInitialDeliverySend(t, qos)

	// QoS0: advance cursor immediately (no ACK expected).
	if qos == 0 {
		return c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}

	// Advance the in-memory cursor after a successful send.
	// This prevents the runner from repeatedly reading and re-sending the same task before ACK arrives.
	// Persistent cursor advancement still happens ONLY on ACK (PUBACK/PUBCOMP).
	*lastTS = t.TS
	*lastTask = t.TaskID
	return nil
}

type deliveryPublishCopy struct {
	packet    *packets.ControlPacket
	publish   *packets.Publish
	fullTopic string
	release   func()
}

type deliveryInflightState struct {
	packetID           uint16
	usedFlowToken      bool
	registeredInflight bool
}

// loadClientDeliveryMessage loads and decodes the stored payload for a delivery task.
// Missing, empty, or undecodable payloads are dropped by advancing the cursor (handled=true)
// so a single bad message cannot stall the queue; transient read errors are returned to retry.
func (c *Client) loadClientDeliveryMessage(
	cursorStore delivery.CursorStore,
	t *store.DeliveryTask,
	lastTS *time.Time,
	lastTask *uuid.UUID,
) (*brokerpublish.Message, bool, error) {
	raw, err := cursorStore.LoadMessagePayload(c.ctx, t.MessageID)
	if err != nil {
		if store.IsNotFound(err) {
			logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Str("message_id", t.MessageID.String()).Msg("drop delivery task because message payload is missing")
			return nil, true, c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
		}
		logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Str("message_id", t.MessageID.String()).Msg("read message payload failed")
		return nil, false, err
	}
	if len(raw) == 0 {
		logger.Logger.Warn().Str("client", c.MetaString()).Str("message_id", t.MessageID.String()).Msg("drop delivery task because message payload is empty")
		return nil, true, c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}

	msg, err := serializer.Serializer.Decode(raw)
	if err != nil || msg == nil || msg.GetPublish() == nil {
		if err == nil {
			err = errors.New("invalid decoded message")
		}
		logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Str("message_id", t.MessageID.String()).Msg("decode message payload failed")
		return nil, false, err
	}
	return msg, false, nil
}

// prepareDeliveryPublishCopy clones the message's PUBLISH into a pooled packet so per-client
// mutations (QoS, PacketID, Retain, subscription IDs) never touch the shared stored message.
// The returned release func must be called to return the packet to the pool.
func (c *Client) prepareDeliveryPublishCopy(msg *brokerpublish.Message) (deliveryPublishCopy, error) {
	pub := msg.GetPublish()
	originalCP := msg.GetControlPacket()
	if originalCP == nil {
		msg.SetPublish(pub)
		originalCP = msg.GetControlPacket()
		if originalCP == nil {
			return deliveryPublishCopy{}, errors.New("failed to get or create ControlPacket")
		}
	}

	newPublishPacket := packetpool.PublishPool.Get()
	packetpool.CopyPublish(newPublishPacket, originalCP)
	publishContent, ok := newPublishPacket.Content.(*packets.Publish)
	if !ok {
		packetpool.PublishPool.Put(newPublishPacket)
		return deliveryPublishCopy{}, errors.New("ControlPacket content is not Publish")
	}
	return deliveryPublishCopy{
		packet:    newPublishPacket,
		publish:   publishContent,
		fullTopic: pub.Topic,
		release: func() {
			packetpool.PublishPool.Put(newPublishPacket)
		},
	}, nil
}

// applyDeliveryPublishOptions applies pre-send rules to the outbound PUBLISH: it drops the
// message (advancing the cursor, handled=true) when expired or carrying an invalid payload
// format, clears Retain unless Retain As Published is set, and attaches subscription IDs.
func (c *Client) applyDeliveryPublishOptions(
	cursorStore delivery.CursorStore,
	t *store.DeliveryTask,
	lastTS *time.Time,
	lastTask *uuid.UUID,
	msg *brokerpublish.Message,
	publishContent *packets.Publish,
) (bool, error) {
	if !applyMessageExpiryForDelivery(publishContent, msg, time.Now()) {
		return true, c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}
	if outboundPayloadFormatInvalid(publishContent) {
		logger.Logger.Warn().Str("client", c.MetaString()).Str("topic", publishContent.Topic).
			Msg("drop stored delivery: payload format indicator=1 but payload is not valid UTF-8")
		return true, c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}
	if !t.RetainAsPublished {
		publishContent.Retain = false
	}
	attachDeliverySubscriptionIDs(publishContent, t.SubscriptionIDs)
	return false, nil
}

func attachDeliverySubscriptionIDs(publishContent *packets.Publish, subIDs []int32) {
	setPublishSubscriptionIdentifiers(publishContent, subIDs)
}

func deliveryQoS(taskQoS int) byte {
	if taskQoS >= 2 {
		return 2
	}
	if taskQoS <= 0 {
		return 0
	}
	return 1
}

// prepareDeliveryQoSAndInflight finalizes a stored PUBLISH before sending: it sets the QoS,
// and for QoS1/QoS2 allocates a PacketID, takes a downlink flow token, and registers the
// outgoing inflight entry. It disconnects the client on PacketID exhaustion or reuse.
func (c *Client) prepareDeliveryQoSAndInflight(
	t *store.DeliveryTask,
	publishContent *packets.Publish,
	qos byte,
) (deliveryInflightState, error) {
	publishContent.QoS = qos
	if qos == 0 {
		publishContent.PacketID = 0
		return deliveryInflightState{}, nil
	}

	state := deliveryInflightState{}
	publishContent.PacketID = c.NextPacketID()
	state.packetID = publishContent.PacketID
	if publishContent.PacketID == 0 {
		_ = c.write(&clientcap.WritePacket{Packet: disconnectForQuotaExceeded("downlink packet identifiers exhausted")})
		_ = c.close()
		return state, errors.New("no available packet id for downlink publish")
	}
	if c.publishBucket != nil {
		c.publishBucket.GetToken(c.ctx)
		state.usedFlowToken = true
	}
	if c.outgoingInflight == nil {
		return state, nil
	}
	replaced := c.outgoingInflight.Put(newDeliveryInflightEntry(t, publishContent.PacketID, qos, state.usedFlowToken))
	if replaced {
		metric.RecordDuplicateDelivery(deliveryPathFromTask(t), "runner")
		if state.usedFlowToken && c.publishBucket != nil {
			c.publishBucket.PutToken()
		}
		_ = c.write(&clientcap.WritePacket{Packet: disconnectForProtocolError("downlink packet id reused while inflight")})
		_ = c.close()
		return state, errors.New("downlink packet id reused while inflight")
	}
	state.registeredInflight = true
	return state, nil
}

// newDeliveryInflightEntry builds the inflight tracking entry for a freshly sent downlink
// QoS1/QoS2 PUBLISH. PersistedInSession is left false: messages first sent in this online
// session are not written to the session store per-message (only snapshotted at disconnect).
func newDeliveryInflightEntry(
	t *store.DeliveryTask,
	packetID uint16,
	qos byte,
	flowTokenHeld bool,
) *outgoingInflightEntry {
	st := outInflightWaitingPubAck
	if qos == 2 {
		st = outInflightWaitingPubRec
	}
	now := time.Now()
	return &outgoingInflightEntry{
		PacketID:      packetID,
		QoS:           qos,
		TaskTS:        t.TS,
		TaskID:        t.TaskID,
		Generation:    t.Generation,
		MessageID:     t.MessageID,
		ShareGroup:    t.ShareGroup,
		SharedTaskID:  t.SharedTaskID,
		FirstPubTime:  now,
		LastSendTime:  now,
		RetryCount:    0,
		FlowTokenHeld: flowTokenHeld,
		State:         st,
	}
}

func observeInitialDeliverySend(t *store.DeliveryTask, qos byte) {
	if t == nil {
		return
	}
	path := deliveryPathFromTask(t)
	metric.RecordDeliveryFirstSendDelay(path, int(qos), time.Since(t.TS))
	metric.RecordDeliveryCursorLag(path, time.Since(t.TS))
	metric.RecordDeliverySendAttempt(path, int(qos), "initial")
}

func deliveryPathFromTask(t *store.DeliveryTask) string {
	if t != nil && (t.ShareGroup != "" || t.SharedTaskID != uuid.Nil) {
		return "shared"
	}
	return "normal"
}

func deliveryPathFromInflight(e *outgoingInflightEntry) string {
	if e != nil && (e.ShareGroup != "" || e.SharedTaskID != uuid.Nil) {
		return "shared"
	}
	return "normal"
}

// handleDeliveryWriteError rolls back inflight/flow-token state after a failed socket write.
// An oversized-publish error is treated as delivered (the packet is dropped) so the cursor
// advances; any other error is returned so the runner retries the task later.
func (c *Client) handleDeliveryWriteError(
	cursorStore delivery.CursorStore,
	t *store.DeliveryTask,
	lastTS *time.Time,
	lastTask *uuid.UUID,
	deliveryPublish deliveryPublishCopy,
	inflight deliveryInflightState,
	err error,
) error {
	c.releaseDeliveryInflight(inflight)
	if IsOversizedOutboundPublish(err) {
		return c.advanceClientDeliveryCursor(cursorStore, t, lastTS, lastTask)
	}
	logger.Logger.Warn().Err(err).Str("client", c.MetaString()).Str("topic", deliveryPublish.fullTopic).Msg("write delivery publish failed")
	return err
}

// releaseDeliveryInflight unwinds a partially established send: it removes the registered
// inflight entry and returns any held downlink flow token back to the bucket.
func (c *Client) releaseDeliveryInflight(inflight deliveryInflightState) {
	if inflight.registeredInflight && c.outgoingInflight != nil {
		if _, ok := c.outgoingInflight.Delete(inflight.packetID); ok && inflight.usedFlowToken && c.publishBucket != nil {
			c.publishBucket.PutToken()
		}
		return
	}
	if inflight.usedFlowToken && c.publishBucket != nil {
		c.publishBucket.PutToken()
	}
}

// advanceClientDeliveryCursor moves both the in-memory and persisted delivery cursor past
// task t and marks any matching shared-subscription task processed. It is used for QoS0
// sends and for messages dropped before delivery, where no ACK will arrive.
func (c *Client) advanceClientDeliveryCursor(
	cursorStore delivery.CursorStore,
	t *store.DeliveryTask,
	lastTS *time.Time,
	lastTask *uuid.UUID,
) error {
	*lastTS = t.TS
	*lastTask = t.TaskID
	if err := cursorStore.AdvanceCursor(c.ctx, store.DeliveryCursor{
		UpdatedTS:  time.Now(),
		ClientID:   c.getID(),
		Generation: t.Generation,
		LastTS:     *lastTS,
		LastTaskID: *lastTask,
	}); err != nil {
		return err
	}
	metric.RecordDeliveryCursorLag(deliveryPathFromTask(t), time.Since(t.TS))
	c.completeSharedDeliveryTask(c.ctx, t.ShareGroup, t.SharedTaskID)
	return nil
}

func (c *Client) completeSharedDeliveryTask(ctx context.Context, shareGroup string, taskID uuid.UUID) {
	if c == nil || c.component == nil || c.component.sharedSubscriptionManager == nil {
		return
	}
	if shareGroup == "" || taskID == uuid.Nil {
		return
	}
	if err := c.component.sharedSubscriptionManager.MarkTaskProcessed(ctx, shareGroup, taskID); err != nil {
		logger.Logger.Warn().Err(err).Str("shareGroup", shareGroup).Str("taskID", taskID.String()).Msg("failed to mark shared task completed")
	}
}
