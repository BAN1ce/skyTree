package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"github.com/google/uuid"
)

func (c *Client) Run(ctx context.Context, handler []Handler) {
	var (
		controlPacket *packets.ControlPacket
		err           error
	)

	ctx = context.WithValue(ctx, pkg.ClientIDKey, c)
	c.ctx, c.cancel = context.WithCancelCause(ctx)
	c.handler = handler

	defer func() {
		// close the client
		if err := c.close(); err != nil {
			logger.Logger.Error().Err(err).Msg("close failed")
		}
	}()

	for {
		select {
		// maybe never trigger, because the client blocked in read
		case <-c.ctx.Done():
			return

		default:
			brokerCfg := c.brokerRuntimeConfig()
			controlPacket, err = wire.Decode(c.conn, wire.DecodeOptions{
				MaxPacketSize: brokerCfg.ConnectAckProperty.MaximumPacketSize,
			})
			if err != nil {
				logger.Logger.Info().Str("client", c.MetaString()).Err(err).Msg("read controlPacket error")
				if c.canSendDisconnect() {
					_ = c.write(&clientcap.WritePacket{Packet: disconnectForReadPacketError(err)})
				} else {
					_ = c.failConnectForReadPacketError(err)
				}
				return
			}

			for _, handler := range c.handler {
				if err := handler.HandlePacket(c.getCtx(), controlPacket, c); err != nil {
					logger.Logger.Info().Str("client", c.MetaString()).Err(err).Msg(fmt.Sprintf("handle packet error, %s", controlPacket.String()))
					return
				}
			}
		}
	}
}

func (c *Client) canSendDisconnect() bool {
	return c != nil && c.connAckAccepted.Load()
}

func (c *Client) closeForProtocolViolation(err error) error {
	if c == nil {
		return err
	}
	if c.cancel != nil {
		c.cancel(err)
	}
	_ = c.close()
	return err
}

// Close closes the client connection and do some clean work.
// It will block until all clean work is done
// If you use ctx to cancel, it will async close the client
func (c *Client) Close() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.close()
}

func (c *Client) CloseWithSessionTakenOver() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.closeWithSessionTakenOverLocked()
}

func (c *Client) closeWithSessionTakenOverLocked() error {
	return c.closeWithDisconnectLocked(disconnectForSessionTakenOver())
}

// closeWithDisconnectLocked 写出 DISCONNECT 后进入完整关闭流程，并抑制遗嘱发布。
// 用于会话接管 / 客户端正常 DISCONNECT(0x00) 等场景：spec 要求不发遗嘱。
func (c *Client) closeWithDisconnectLocked(packet *packets.ControlPacket) error {
	c.disconnectWithoutWill.Store(packets.DisconnectNormalDisconnection)
	return c.writeDisconnectAndCloseLocked(packet)
}

// closeWithDisconnectKeepWillLocked 写出 DISCONNECT 后进入完整关闭流程，但保留遗嘱发布逻辑。
// 用于服务端因 KeepAlive 超时 / 关停 / 协议错误 等异常关闭场景：spec 要求发遗嘱。
func (c *Client) closeWithDisconnectKeepWillLocked(packet *packets.ControlPacket) error {
	return c.writeDisconnectAndCloseLocked(packet)
}

// writeDisconnectAndCloseLocked 会先尽力写出 DISCONNECT，再执行最终关闭流程。
// 调用方需要持有 c.mux，避免关闭期间还有其它 goroutine 修改 client 状态。
func (c *Client) writeDisconnectAndCloseLocked(packet *packets.ControlPacket) error {
	var writeErr error
	if packet != nil && c.canSendDisconnect() {
		c.writeMux.Lock()
		writeErr = c.write(&clientcap.WritePacket{Packet: packet})
		c.writeMux.Unlock()
	}
	return errors.Join(writeErr, c.close())
}

// CloseWithDisconnectKeepWill 由 broker 等外部调用方触发：写出 DISCONNECT 后关闭连接，
// 不抑制遗嘱发布。
func (c *Client) CloseWithDisconnectKeepWill(packet *packets.ControlPacket) error {
	if c == nil {
		return nil
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.closeWithDisconnectKeepWillLocked(packet)
}

// CloseWithDisconnectNoWill sends a server DISCONNECT and closes the connection without publishing Will.
// Use it for graceful server-side shutdown paths where the broker explicitly tells the client it is closing.
func (c *Client) CloseWithDisconnectNoWill(packet *packets.ControlPacket) error {
	if c == nil {
		return nil
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.closeWithDisconnectLocked(packet)
}

func (c *Client) collectOutgoingUnfinishedMessages() []*proto_session.UnfinishedMessage {
	if c == nil || c.outgoingInflight == nil {
		return nil
	}
	entries := c.outgoingInflight.All()
	if len(entries) == 0 {
		return nil
	}

	out := make([]*proto_session.UnfinishedMessage, 0, len(entries))
	nowUS := time.Now().UnixMicro()
	for _, e := range entries {
		if e == nil || e.PacketID == 0 {
			continue
		}
		if e.QoS == 1 && !e.Retained {
			continue
		}
		st := proto_session.UnfinishedMessage_WAITING_PUBACK
		switch e.QoS {
		case 1:
			if !e.Retained {
				continue
			}
			st = proto_session.UnfinishedMessage_WAITING_PUBACK
		case 2:
			if e.State == outInflightWaitingPubComp {
				st = proto_session.UnfinishedMessage_WAITING_PUBCOMP
			} else {
				st = proto_session.UnfinishedMessage_WAITING_PUBREC
			}
		default:
			continue
		}
		firstUS := nowUS
		if !e.FirstPubTime.IsZero() {
			firstUS = e.FirstPubTime.UnixMicro()
		}
		out = append(out, &proto_session.UnfinishedMessage{
			MessageID:     e.MessageID.String(),
			PacketID:      uint32(e.PacketID),
			Qos:           uint32(e.QoS),
			State:         st,
			IsOutgoing:    true,
			FirstPubTime:  firstUS,
			PublishPacket: append([]byte(nil), e.PublishPacket...),
		})
	}
	if len(out) > 0 {
		logger.Logger.Debug().
			Str("client", c.getID()).
			Int("count", len(out)).
			Msg("saving outgoing unfinished messages")
	}
	return out
}

func (c *Client) collectOutgoingReplayCursor() *proto_session.OutgoingReplayCursor {
	if c == nil || c.outgoingInflight == nil {
		return nil
	}
	entries := c.outgoingInflight.All()
	if len(entries) == 0 {
		return nil
	}

	var earliest *outgoingInflightEntry
	for _, e := range entries {
		if e == nil || e.Acked || e.Retained || e.QoS != 1 || e.TaskID == uuid.Nil {
			continue
		}
		if earliest == nil || outgoingInflightLess(e, earliest) {
			cp := *e
			earliest = &cp
		}
	}
	if earliest == nil {
		return nil
	}
	logger.Logger.Debug().
		Str("client", c.getID()).
		Str("task_id", earliest.TaskID.String()).
		Int64("task_unix_micro", earliest.TaskTS.UnixMicro()).
		Msg("saving outgoing qos1 replay cursor")
	return &proto_session.OutgoingReplayCursor{
		TaskUnixMicro: earliest.TaskTS.UnixMicro(),
		TaskID:        earliest.TaskID.String(),
		Generation:    earliest.Generation,
	}
}

func (c *Client) persistUnfinishedMessagesToSession(ctx context.Context) {
	if c == nil || c.component == nil || c.component.sessionCenter == nil {
		return
	}
	if c.getID() == "" {
		return
	}

	var allUnfinished []*proto_session.UnfinishedMessage
	if outgoing := c.collectOutgoingUnfinishedMessages(); len(outgoing) > 0 {
		allUnfinished = append(allUnfinished, outgoing...)
	}
	if allUnfinished == nil {
		allUnfinished = make([]*proto_session.UnfinishedMessage, 0)
	}
	outgoingReplayCursor := c.collectOutgoingReplayCursor()

	disconnectFlag := c.disconnectWithoutWill.Load()
	clearWill := disconnectFlag != 4 && disconnectFlag != -1

	if c.component.stateRouter == nil {
		logger.Logger.Error().
			Str("client", c.getID()).
			Msg("failed to save unfinished messages: state router is nil")
		return
	}
	if err := c.component.stateRouter.SaveOfflineState(ctx, staterouter.SaveOfflineStateRequest{
		BrokerNodeID:          c.clusterRuntimeConfig().LocalNodeID,
		ClientID:              c.getID(),
		OwnerToken:            c.getOwnerToken(),
		UnfinishedMessages:    allUnfinished,
		OutgoingReplayCursor:  outgoingReplayCursor,
		ClearWill:             clearWill,
		SessionExpiryInterval: c.sessionExpiryInterval,
		NowUnixNano:           time.Now().UnixNano(),
	}); err != nil {
		logger.Logger.Error().
			Str("client", c.getID()).
			Err(err).
			Int("unfinished_count", len(allUnfinished)).
			Bool("has_outgoing_replay_cursor", outgoingReplayCursor != nil).
			Msg("failed to save unfinished messages to session")
	} else {
		logger.Logger.Info().
			Str("client", c.getID()).
			Int("unfinished_count", len(allUnfinished)).
			Bool("has_outgoing_replay_cursor", outgoingReplayCursor != nil).
			Bool("clear_will", clearWill).
			Msg("successfully saved unfinished messages to session")
	}
}

func (c *Client) cleanupKeepAlive(ctx context.Context) {
	if c == nil || c.component == nil || c.component.keepAliveTracker == nil {
		return
	}
	// Old connections can close after a new owner for the same clientID is online.
	// The owner token fence prevents stale close cleanup from deleting the new keepalive entry.
	c.component.keepAliveTracker.DeleteIfOwner(c.getID(), c.getOwnerToken())
}

func shouldPublishWillForDisconnect(reason int64) bool {
	return reason == 4 || reason == -1
}

func disconnectForReadPacketError(err error) *packets.ControlPacket {
	var wireErr *wire.WireError
	if errors.As(err, &wireErr) {
		switch wireErr.Kind {
		case wire.ErrPacketTooLarge:
			return disconnectForPacketTooLarge(wireErr.PacketSize, wireErr.MaxPacketSize)
		case wire.ErrMalformedPacket:
			return newServerDisconnect(packets.DisconnectMalformedPacket, wireErr.Error())
		case wire.ErrProtocolError:
			return newServerDisconnect(packets.DisconnectProtocolError, wireErr.Error())
		case wire.ErrImplementation:
			return newServerDisconnect(packets.DisconnectImplementationSpecificError, wireErr.Error())
		default:
			return newServerDisconnect(packets.DisconnectProtocolError, wireErr.Error())
		}
	}
	return disconnectForProtocolError(err.Error())
}

func sessionExpiryDuration(interval uint32) (time.Duration, bool) {
	switch interval {
	case mqttNeverExpireSessionInterval:
		return 0, false
	default:
		return time.Duration(interval) * time.Second, true
	}
}

func (c *Client) cleanupExpiredSessionState(ctx context.Context, deleteSession bool) {
	if c == nil || c.component == nil || c.getID() == "" {
		return
	}
	if c.component.subCenter != nil {
		if _, err := c.component.subCenter.DeleteClient(ctx, &proto_topic.DeleteClientRequest{
			ClientID:   c.getID(),
			OwnerToken: c.getOwnerToken(),
		}); err != nil {
			logger.Logger.Error().
				Err(err).
				Str("client", c.getID()).
				Msg("failed to delete subscriptions for expired session")
		}
	}
	c.deleteClientDeliveryState(ctx, "expired session")
	if deleteSession && c.component.sessionCenter != nil {
		if err := c.component.sessionCenter.DeleteSession(ctx, &proto_session.DeleteSessionRequest{ClientID: c.getID()}); err != nil {
			logger.Logger.Error().
				Err(err).
				Str("client", c.getID()).
				Msg("failed to delete expired session")
		}
	}
}

func (c *Client) deleteClientDeliveryState(ctx context.Context, reason string) {
	if c == nil || c.component == nil || c.component.deliveryCursorStore == nil || c.getID() == "" {
		return
	}
	deleter, ok := c.component.deliveryCursorStore.(delivery.ClientStateDeleter)
	if !ok {
		return
	}
	if err := deleter.DeleteClientState(ctx, c.getID()); err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client", c.getID()).
			Str("reason", reason).
			Msg("failed to delete client delivery state")
	}
}

func shouldExpireClosedClient(ctx context.Context, center session.Center, clientID, ownerToken string) bool {
	if center == nil || clientID == "" || ownerToken == "" {
		return true
	}
	resp, err := center.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{ClientID: clientID})
	if err != nil || resp == nil || !resp.GetExist() || resp.GetOwner() == nil {
		return true
	}
	owner := resp.GetOwner()
	return !owner.GetOnline() && owner.GetOwnerToken() == ownerToken
}

func (c *Client) brokerLifecycleContext() context.Context {
	if c != nil && c.component != nil && c.component.lifecycleCtx != nil {
		return c.component.lifecycleCtx
	}
	return context.Background()
}

func (c *Client) scheduleLifecycleTimerTask(delay time.Duration, task func(context.Context)) {
	if c == nil || task == nil || delay < 0 {
		return
	}
	lifecycleCtx := c.brokerLifecycleContext()
	var wg *sync.WaitGroup
	if c.component != nil {
		wg = c.component.backgroundTaskWG
	}
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-lifecycleCtx.Done():
			return
		}
		taskCtx, cancel := context.WithTimeout(lifecycleCtx, 10*time.Second)
		defer cancel()
		task(taskCtx)
	}()
}

func (c *Client) scheduleSessionExpiryCleanup(publishWillAtExpiry bool) {
	if c == nil || c.component == nil {
		return
	}
	expiry, expires := sessionExpiryDuration(c.sessionExpiryInterval)
	if !expires {
		return
	}
	if expiry == 0 {
		ctx, cancel := context.WithTimeout(c.brokerLifecycleContext(), 10*time.Second)
		defer cancel()
		c.cleanupExpiredSessionState(ctx, true)
		return
	}
	// When Will publication is scheduled at session-expiry through willDelayCenter,
	// keep a short grace window before local cleanup so scanner-based publication
	// can observe session state first.
	if c.willScheduledAtSessionExpiry.Load() {
		expiry += 2 * time.Second
	}

	component := c.component
	clientID := c.getID()
	ownerToken := c.getOwnerToken()
	publisher := willMessagePublisher{
		ch:         component.notifyWillMessageChan,
		clientID:   clientID,
		ownerToken: ownerToken,
	}
	willDelay := time.Duration(0)
	var publishContent *packets.Publish
	if publishWillAtExpiry && c.willMessage != nil {
		publishContent = c.willMessage.GetPublish()
		willDelay = c.willMessage.WillDelay
	}

	c.scheduleLifecycleTimerTask(expiry, func(ctx context.Context) {
		if !shouldExpireClosedClient(ctx, component.sessionCenter, clientID, ownerToken) {
			return
		}
		publisher.publish(ctx, publishContent, willDelay)

		tmp := &Client{
			ID:         clientID,
			ownerToken: ownerToken,
			component:  component,
		}
		tmp.cleanupExpiredSessionState(ctx, true)
	})
}

func (c *Client) handleWillOnClose(ctx context.Context) bool {
	publishContent, willDelay, ok := c.willPublishOnClose(ctx)
	if !ok {
		return false
	}
	if !c.hasOwnerTokenForWill("publish on close") {
		return false
	}

	expiry, expires := sessionExpiryDuration(c.sessionExpiryInterval)
	if expires && expiry == 0 {
		c.willPublisher().publish(ctx, publishContent, willDelay)
		return false
	}
	if shouldPublishWillAtSessionExpiry(willDelay, expiry, expires) {
		if c.scheduleWillAtSessionExpiryTask(ctx, expiry) {
			c.willScheduledAtSessionExpiry.Store(true)
			return false
		}
		return true
	}
	if c.scheduleWillDelay(ctx, willDelay) {
		return false
	}

	// No delay: publish immediately.
	if willDelay <= 0 {
		c.willPublisher().publish(ctx, publishContent, willDelay)
		return false
	}

	logger.Logger.Error().
		Str("client", c.getID()).
		Dur("will_delay", willDelay).
		Msg("failed to schedule delayed will message")
	return false
}

func (c *Client) willPublishOnClose(ctx context.Context) (*packets.Publish, time.Duration, bool) {
	if c == nil || c.component == nil || c.willMessage == nil {
		return nil, 0, false
	}
	publishContent := c.willMessage.GetPublish()
	if publishContent == nil || !c.connAckAccepted.Load() {
		return nil, 0, false
	}
	if !shouldPublishWillForDisconnect(c.disconnectWithoutWill.Load()) {
		return nil, 0, false
	}
	c.hydrateWillPropertiesFromSession(ctx, publishContent)
	return publishContent, c.willMessage.WillDelay, true
}

func (c *Client) hydrateWillPropertiesFromSession(ctx context.Context, publishContent *packets.Publish) {
	if publishContent.Properties != nil || c.component.sessionCenter == nil {
		return
	}
	sessionResp, err := c.component.sessionCenter.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID: c.getID(),
	})
	if err != nil || sessionResp == nil || !sessionResp.Exist || sessionResp.Session == nil {
		return
	}
	willMsg := sessionResp.Session.GetWillMessage()
	if willMsg == nil {
		return
	}
	if props := session.ProtoToWillProperties(willMsg); props != nil {
		publishContent.Properties = props
	}
}

func shouldPublishWillAtSessionExpiry(willDelay, expiry time.Duration, expires bool) bool {
	return willDelay > 0 && expires && expiry <= willDelay
}

func (c *Client) hasOwnerTokenForWill(operation string) bool {
	if c == nil || c.getOwnerToken() == "" {
		logger.Logger.Error().
			Str("operation", operation).
			Msg("cannot schedule will without owner token")
		return false
	}
	return true
}

func (c *Client) scheduleWillDelay(ctx context.Context, willDelay time.Duration) bool {
	if willDelay <= 0 || c == nil || c.component == nil {
		return false
	}
	if !c.hasOwnerTokenForWill("will delay") {
		return false
	}
	ownerToken := c.getOwnerToken()
	if c.component.willDelayCenter == nil {
		return c.scheduleLocalWillDelayFallback(willDelay)
	}
	scheduledPublishTime := time.Now().UnixMicro() + int64(willDelay/time.Microsecond)
	if err := c.component.willDelayCenter.AddTask(ctx, &proto_will_delay.WillDelayTask{
		ClientID:             c.getID(),
		ScheduledPublishTime: scheduledPublishTime,
		OwnerToken:           ownerToken,
	}); err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client", c.getID()).
			Dur("will_delay", willDelay).
			Msg("failed to add will delay task")
		return c.scheduleLocalWillDelayFallback(willDelay)
	}
	logger.Logger.Info().
		Str("client", c.getID()).
		Dur("will_delay", willDelay).
		Int64("scheduled_time", scheduledPublishTime).
		Msg("added will delay task")
	return true
}

func (c *Client) scheduleWillAtSessionExpiryTask(ctx context.Context, expiry time.Duration) bool {
	if c == nil || c.component == nil || expiry <= 0 || c.component.willDelayCenter == nil {
		return false
	}
	if !c.hasOwnerTokenForWill("will at session expiry") {
		return false
	}
	ownerToken := c.getOwnerToken()
	scheduledPublishTime := time.Now().UnixMicro() + int64(expiry/time.Microsecond)
	if err := c.component.willDelayCenter.AddTask(ctx, &proto_will_delay.WillDelayTask{
		ClientID:             c.getID(),
		ScheduledPublishTime: scheduledPublishTime,
		OwnerToken:           ownerToken,
	}); err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client", c.getID()).
			Dur("session_expiry", expiry).
			Msg("failed to add will task scheduled at session expiry")
		return false
	}
	logger.Logger.Info().
		Str("client", c.getID()).
		Dur("session_expiry", expiry).
		Int64("scheduled_time", scheduledPublishTime).
		Msg("added will task scheduled at session expiry")
	return true
}

func (c *Client) scheduleLocalWillDelayFallback(willDelay time.Duration) bool {
	if c == nil || c.component == nil || willDelay <= 0 || c.component.notifyWillMessageChan == nil {
		return false
	}
	component := c.component
	clientID := c.getID()
	ownerToken := c.getOwnerToken()
	publisher := willMessagePublisher{
		ch:         component.notifyWillMessageChan,
		clientID:   clientID,
		ownerToken: ownerToken,
	}
	var publishContent *packets.Publish
	if c.willMessage != nil {
		publishContent = clonePublish(c.willMessage.GetPublish())
	}
	c.scheduleLifecycleTimerTask(willDelay, func(ctx context.Context) {
		if !shouldExpireClosedClient(ctx, component.sessionCenter, clientID, ownerToken) {
			return
		}
		if component.sessionCenter == nil {
			publisher.publish(ctx, publishContent, willDelay)
			return
		}
		sessionResp, err := component.sessionCenter.GetSession(ctx, &proto_session.ReadSessionRequest{
			ClientID: clientID,
		})
		if err != nil || sessionResp == nil || !sessionResp.GetExist() || sessionResp.GetSession() == nil {
			if err != nil {
				logger.Logger.Warn().
					Err(err).
					Str("client", clientID).
					Msg("failed to load session will for delayed fallback; using in-memory will")
			} else {
				logger.Logger.Warn().
					Str("client", clientID).
					Msg("session will missing for delayed fallback; using in-memory will")
			}
			publisher.publish(ctx, publishContent, willDelay)
			return
		}
		willMsg := sessionResp.GetSession().GetWillMessage()
		if willMsg == nil {
			publisher.publish(ctx, publishContent, willDelay)
			return
		}
		publish := &packets.Publish{
			Topic:   willMsg.GetTopic(),
			Payload: willMsg.GetPayload(),
			QoS:     byte(willMsg.GetQos()),
			Retain:  willMsg.GetRetain(),
		}
		if props := session.ProtoToWillProperties(willMsg); props != nil {
			publish.Properties = props
		}
		publisher.publish(ctx, publish, willDelay)
	})
	logger.Logger.Warn().
		Str("client", c.getID()).
		Dur("will_delay", willDelay).
		Msg("using local will delay fallback timer")
	return true
}

// close 只执行一次 client 最终关闭流程。
// 关闭流程包括：取消 client context、通知共享订阅和投递事件下线、关闭底层 socket、
// 持久化未完成 QoS 状态、清理 keepalive 状态、处理遗嘱消息和 session 过期任务。
// 外层 Close/CloseWith... 路径会在持有 c.mux 时调用它，这是有意的最终状态栅栏，
// 用于阻止关闭期间还有其它路径并发修改 client 状态。
func (c *Client) close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		if logger.Logger != nil {
			logger.Logger.Info().Str("client", c.MetaString()).Msg("close client")
		}
		if c.cancel != nil {
			c.cancel(errors.New("client closed"))
		}
		baseCtx := c.brokerLifecycleContext()

		if c.component != nil && c.component.sharedSubscriptionManager != nil && c.ID != "" {
			offlineCtx, offlineCancel := context.WithTimeout(baseCtx, 3*time.Second)
			err := c.component.sharedSubscriptionManager.OnClientOffline(offlineCtx, c.getID())
			offlineCancel()
			if err != nil {
				if logger.Logger != nil {
					logger.Logger.Warn().Err(err).Str("client", c.getID()).Msg("failed to notify shared subscription offline")
				}
			}
		}

		if c.component != nil && c.component.clientDeliveryEvent != nil && c.deliveryListenerID != "" && c.ID != "" {
			listenerCtx, listenerCancel := context.WithTimeout(baseCtx, 3*time.Second)
			_ = c.component.clientDeliveryEvent.DeleteListener(listenerCtx, c.ID, c.deliveryListenerID)
			listenerCancel()
			c.deliveryListenerID = ""
		}

		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				closeErr = err
				if logger.Logger != nil {
					logger.Logger.Info().Str("client", c.MetaString()).Err(err).Msg("close conn error")
				}
			}
		}

		var (
			ctx, cancel = context.WithTimeout(baseCtx, 10*time.Second)
		)
		defer cancel()

		c.persistUnfinishedMessagesToSession(ctx)
		c.cleanupKeepAlive(ctx)
		publishWillAtExpiry := c.handleWillOnClose(ctx)
		c.scheduleSessionExpiryCleanup(publishWillAtExpiry)

	})
	return closeErr
}
