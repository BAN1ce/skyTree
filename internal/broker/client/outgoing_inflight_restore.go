package client

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

// restoreOutgoingInflightFromSession best-effort restores all persisted downlink inflight messages and retransmits them.
// It is used on reconnect when CleanStart=false and SessionPresent=true.
func (c *Client) restoreOutgoingInflightFromSession(ctx context.Context, sess *proto_session.Session) {
	if c == nil || sess == nil {
		return
	}
	c.restoreQoS1ReplayCursorFromSession(ctx, sess)
	for _, unfinished := range outgoingUnfinishedMessages(sess.GetUnfinishedMessages()) {
		if c.ctx != nil && c.ctx.Err() != nil {
			return
		}
		c.restoreOneOutgoingInflight(ctx, unfinished)
	}
}

func (c *Client) restoreQoS1ReplayCursorFromSession(ctx context.Context, sess *proto_session.Session) {
	if c == nil || sess == nil {
		return
	}
	cursor := sess.GetOutgoingReplayCursor()
	if cursor == nil {
		c.outgoingReplayCursor = nil
		return
	}
	taskID, err := uuid.Parse(cursor.GetTaskID())
	if err != nil || taskID == uuid.Nil || cursor.GetTaskUnixMicro() <= 0 {
		logger.Logger.Warn().
			Str("client", c.metaString()).
			Str("task_id", cursor.GetTaskID()).
			Int64("task_unix_micro", cursor.GetTaskUnixMicro()).
			Msg("restore qos1 replay cursor: invalid cursor")
		c.outgoingReplayCursor = nil
		return
	}
	if c.component == nil || c.component.deliveryCursorStore == nil {
		logger.Logger.Warn().
			Str("client", c.metaString()).
			Str("task_id", taskID.String()).
			Msg("restore qos1 replay cursor: missing delivery cursor store")
		c.outgoingReplayCursor = nil
		return
	}
	if persisted, err := c.component.deliveryCursorStore.ReadCursor(ctx, c.getID()); err == nil && persisted != nil {
		if persisted.Generation != cursor.GetGeneration() {
			logger.Logger.Warn().
				Str("client", c.metaString()).
				Int64("cursor_generation", cursor.GetGeneration()).
				Int64("delivery_generation", persisted.Generation).
				Msg("restore qos1 replay cursor: generation mismatch")
			c.outgoingReplayCursor = nil
			return
		}
	} else if err != nil {
		logger.Logger.Warn().
			Err(err).
			Str("client", c.metaString()).
			Str("task_id", taskID.String()).
			Msg("restore qos1 replay cursor: read delivery cursor failed")
		c.outgoingReplayCursor = nil
		return
	}
	c.outgoingReplayCursor = &outgoingReplayCursor{
		TaskTS:     time.UnixMicro(cursor.GetTaskUnixMicro()),
		TaskID:     taskID,
		Generation: cursor.GetGeneration(),
	}
}

func outgoingUnfinishedMessages(msgs []*proto_session.UnfinishedMessage) []*proto_session.UnfinishedMessage {
	out := make([]*proto_session.UnfinishedMessage, 0, len(msgs))
	for _, m := range msgs {
		if !isRestorableOutgoingUnfinished(m) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i].GetFirstPubTime(), out[j].GetFirstPubTime()
		if left != right {
			if left == 0 {
				return false
			}
			if right == 0 {
				return true
			}
			return left < right
		}
		return out[i].GetPacketID() < out[j].GetPacketID()
	})
	return out
}

func isRestorableOutgoingUnfinished(m *proto_session.UnfinishedMessage) bool {
	if m == nil || !m.GetIsOutgoing() {
		return false
	}
	if m.GetPacketID() == 0 {
		return false
	}
	switch m.GetQos() {
	case 1:
		return len(m.GetPublishPacket()) > 0
	case 2:
	default:
		return false
	}
	switch m.GetState() {
	case proto_session.UnfinishedMessage_WAITING_PUBACK,
		proto_session.UnfinishedMessage_WAITING_PUBREC,
		proto_session.UnfinishedMessage_WAITING_PUBCOMP:
		return true
	default:
		return false
	}
}

func (c *Client) restoreOneOutgoingInflight(ctx context.Context, unfinished *proto_session.UnfinishedMessage) {
	if c == nil || unfinished == nil {
		return
	}
	if len(unfinished.GetPublishPacket()) > 0 {
		if err := c.restoreRetainedOutgoingInflight(unfinished); err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.metaString()).Str("message_id", unfinished.GetMessageID()).Msg("restore outgoing retained inflight failed")
		}
		return
	}

	msgID, err := uuid.Parse(unfinished.GetMessageID())
	if err != nil || msgID == uuid.Nil {
		logger.Logger.Warn().Str("client", c.metaString()).Str("message_id", unfinished.GetMessageID()).Msg("restore outgoing inflight: invalid message_id")
		return
	}
	if c.component == nil || c.component.deliveryCursorStore == nil {
		logger.Logger.Warn().Str("client", c.metaString()).Str("message_id", msgID.String()).Msg("restore outgoing inflight: missing delivery cursor store")
		return
	}
	t, err := c.findDeliveryTaskByMessageID(ctx, msgID)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("client", c.metaString()).Str("message_id", msgID.String()).Msg("restore outgoing inflight: delivery task not found")
		return
	}

	st, ok := outgoingInflightStateFromSession(unfinished.GetState())
	if !ok {
		return
	}
	packetID := uint16(unfinished.GetPacketID())
	qos := byte(unfinished.GetQos())
	first := firstPublishTimeFromSession(unfinished)
	now := time.Now()

	entry := &outgoingInflightEntry{
		PacketID:     packetID,
		QoS:          qos,
		TaskTS:       t.TS,
		TaskID:       t.TaskID,
		Generation:   t.Generation,
		MessageID:    t.MessageID,
		FirstPubTime: first,
		RetryCount:   0,
		State:        st,
		// Restored from the session store, so the entry exists in session and its
		// terminal ack must issue a RemoveOutgoingUnfinished to clean it up.
		PersistedInSession: true,
	}
	if !c.registerRestoredOutgoingInflight(entry) {
		return
	}
	entry.LastSendTime = now

	// Retransmit based on state machine stage:
	// - WAITING_PUBACK / WAITING_PUBREC: retransmit PUBLISH with DUP=1 and the same PacketID.
	// - WAITING_PUBCOMP: retransmit PUBREL with the required fixed header flags.
	if unfinished.GetState() == proto_session.UnfinishedMessage_WAITING_PUBCOMP {
		_ = c.Write(&clientcap.WritePacket{Packet: newPubrelForRestore(packetID)})
		return
	}

	if err := c.retransmitPublishFromTask(ctx, t, qos, packetID); err != nil {
		logger.Logger.Warn().Err(err).Str("client", c.metaString()).Str("message_id", msgID.String()).Msg("restore outgoing inflight: retransmit publish failed")
	}
}

func (c *Client) restoreRetainedOutgoingInflight(unfinished *proto_session.UnfinishedMessage) error {
	cp, err := wire.Decode(bytes.NewBuffer(unfinished.GetPublishPacket()), wire.DecodeOptions{})
	if err != nil {
		return err
	}
	publish, ok := cp.Content.(*packets.Publish)
	if !ok || publish == nil {
		return errors.New("retained unfinished packet is not PUBLISH")
	}
	qos := byte(unfinished.GetQos())
	if qos != 1 && qos != 2 {
		return errors.New("invalid retained qos")
	}
	packetID := uint16(unfinished.GetPacketID())
	publish.PacketID = packetID
	publish.QoS = qos
	publish.Duplicate = true

	st, ok := outgoingInflightStateFromSession(unfinished.GetState())
	if !ok {
		return errors.New("invalid retained inflight state")
	}
	messageID, err := uuid.Parse(unfinished.GetMessageID())
	if err != nil || messageID == uuid.Nil {
		messageID = uuid.New()
	}
	entry := &outgoingInflightEntry{
		PacketID:      packetID,
		QoS:           qos,
		TaskTS:        firstPublishTimeFromSession(unfinished),
		TaskID:        messageID,
		MessageID:     messageID,
		Retained:      true,
		PublishPacket: append([]byte(nil), unfinished.GetPublishPacket()...),
		FirstPubTime:  firstPublishTimeFromSession(unfinished),
		State:         st,
		// Restored from the session store, so its terminal ack must clean it up there.
		PersistedInSession: true,
	}
	if !c.registerRestoredOutgoingInflight(entry) {
		return nil
	}
	entry.LastSendTime = time.Now()

	if unfinished.GetState() == proto_session.UnfinishedMessage_WAITING_PUBCOMP {
		return c.Write(&clientcap.WritePacket{Packet: newPubrelForRestore(packetID)})
	}
	return c.Write(&clientcap.WritePacket{Packet: cp, FullTopic: publish.Topic})
}

func (c *Client) retransmitRetainedPublishFromEntry(e *outgoingInflightEntry) error {
	if c == nil || e == nil || len(e.PublishPacket) == 0 {
		return errors.New("missing retained publish packet")
	}
	cp, err := wire.Decode(bytes.NewBuffer(e.PublishPacket), wire.DecodeOptions{})
	if err != nil {
		return err
	}
	publish, ok := cp.Content.(*packets.Publish)
	if !ok || publish == nil {
		return errors.New("retained inflight packet is not PUBLISH")
	}
	publish.PacketID = e.PacketID
	publish.QoS = e.QoS
	publish.Duplicate = true
	return c.Write(&clientcap.WritePacket{Packet: cp, FullTopic: publish.Topic})
}

func (c *Client) registerRestoredOutgoingInflight(entry *outgoingInflightEntry) bool {
	if c == nil || entry == nil {
		return false
	}
	tokenHeld := true
	if c.publishBucket != nil {
		tokenHeld = c.publishBucket.TryGetToken()
	}
	entry.FlowTokenHeld = tokenHeld
	if !tokenHeld {
		entry.LastSendTime = time.Time{}
	}
	if c.outgoingInflight != nil {
		c.outgoingInflight.Put(entry)
	}
	return tokenHeld
}

func outgoingInflightStateFromSession(st proto_session.UnfinishedMessage_MessageState) (outgoingInflightState, bool) {
	switch st {
	case proto_session.UnfinishedMessage_WAITING_PUBACK:
		return outInflightWaitingPubAck, true
	case proto_session.UnfinishedMessage_WAITING_PUBREC:
		return outInflightWaitingPubRec, true
	case proto_session.UnfinishedMessage_WAITING_PUBCOMP:
		return outInflightWaitingPubComp, true
	default:
		return 0, false
	}
}

func firstPublishTimeFromSession(m *proto_session.UnfinishedMessage) time.Time {
	if m != nil && m.GetFirstPubTime() > 0 {
		return time.UnixMicro(m.GetFirstPubTime())
	}
	return time.Now()
}

func newPubrelForRestore(packetID uint16) *packets.ControlPacket {
	pubRelCP := packets.NewControlPacket(packets.PUBREL)
	pubRelCP.Content = &packets.Pubrel{PacketID: packetID, ReasonCode: 0}
	return pubRelCP
}

func (c *Client) findDeliveryTaskByMessageID(ctx context.Context, messageID uuid.UUID) (*storeDeliveryTask, error) {
	if c == nil || c.component == nil || c.component.deliveryCursorStore == nil {
		return nil, errors.New("missing delivery cursor store")
	}
	clientID := c.getID()
	if clientID == "" {
		return nil, errors.New("empty client id")
	}
	cursorStore := c.component.deliveryCursorStore

	lastTS := time.UnixMicro(0)
	lastTask := uuid.Nil
	if cur, err := cursorStore.ReadCursor(ctx, clientID); err == nil && cur != nil {
		lastTS = cur.LastTS
		lastTask = cur.LastTaskID
	}

	// Read a small batch from the persisted cursor; the inflight task should be at the head.
	var tasks []*store.DeliveryTask
	tasks, err := cursorStore.ReadTasks(ctx, clientID, lastTS, lastTask, 200)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if t.MessageID == messageID {
			return &storeDeliveryTask{
				TS:                t.TS,
				TaskID:            t.TaskID,
				MessageID:         t.MessageID,
				Generation:        t.Generation,
				SubscriptionIDs:   t.SubscriptionIDs,
				RetainAsPublished: t.RetainAsPublished,
			}, nil
		}
	}
	return nil, errors.New("not found")
}

type storeDeliveryTask struct {
	TS         time.Time
	TaskID     uuid.UUID
	MessageID  uuid.UUID
	Generation int64

	SubscriptionIDs   []int32
	RetainAsPublished bool
}

func (c *Client) retransmitPublishFromTask(ctx context.Context, t *storeDeliveryTask, qos byte, packetID uint16) error {
	if err := validateRetransmitPublishArgs(c, t, qos, packetID); err != nil {
		return err
	}
	msg, err := c.loadRetransmitPublishMessage(ctx, t)
	if err != nil {
		return err
	}
	deliveryPublish, err := c.prepareDeliveryPublishCopy(msg)
	if err != nil {
		return err
	}
	defer deliveryPublish.release()

	applyRetransmitPublishOptions(deliveryPublish.publish, t, qos, packetID)

	return c.Write(&clientcap.WritePacket{
		Packet:    deliveryPublish.packet,
		FullTopic: deliveryPublish.fullTopic,
	})
}

func validateRetransmitPublishArgs(c *Client, t *storeDeliveryTask, qos byte, packetID uint16) error {
	if c == nil || t == nil || t.MessageID == uuid.Nil {
		return errors.New("invalid args")
	}
	if qos != 1 && qos != 2 {
		return errors.New("invalid qos")
	}
	if packetID == 0 {
		return errors.New("invalid packet id")
	}
	return nil
}

func (c *Client) loadRetransmitPublishMessage(ctx context.Context, t *storeDeliveryTask) (*brokerpublish.Message, error) {
	raw, err := c.component.deliveryCursorStore.LoadMessagePayload(ctx, t.MessageID)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("empty message payload")
	}
	msg, err := serializer.Serializer.Decode(raw)
	if err != nil {
		return nil, err
	}
	if msg == nil || msg.GetPublish() == nil {
		return nil, errors.New("invalid decoded message")
	}
	return msg, nil
}

func applyRetransmitPublishOptions(publishContent *packets.Publish, t *storeDeliveryTask, qos byte, packetID uint16) {
	if !t.RetainAsPublished {
		publishContent.Retain = false
	}
	attachDeliverySubscriptionIDs(publishContent, t.SubscriptionIDs)
	publishContent.QoS = qos
	publishContent.PacketID = packetID
	publishContent.Duplicate = true
}
