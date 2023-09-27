package qos2

import (
	"context"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/util"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

type Option func(q *QoS2)

func WithPublishWriter(writer broker.PublishWriter) Option {
	return func(q *QoS2) {
		q.meta.writer = writer
	}
}

func WithSession(session Session) Option {
	return func(q *QoS2) {
		q.session = session
	}
}

func WithSubOption(option *proto.SubOption) Option {
	return func(q *QoS2) {
		q.meta.subOption = option
	}
}

type QoS2 struct {
	ctx         context.Context
	cancel      context.CancelFunc
	meta        *meta
	queue       *Queue
	publishChan chan *packet.PublishMessage
	session     Session
}

func NewQos2(topic string, options ...Option) *QoS2 {

	var (
		latestMessageID string
	)
	t := &QoS2{
		meta: &meta{
			topic:           topic,
			latestMessageID: latestMessageID,
		},
	}
	for _, op := range options {
		op(t)
	}
	t.queue = NewQoS2Queue(t.meta.writer)
	t.meta.latestMessageID, _ = t.session.ReadTopicLatestPushedMessageID(topic)
	return t
}

func (q *QoS2) Start(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
	if q.meta.windowSize == 0 {
		q.meta.windowSize = config.GetTopic().WindowSize
	}
	q.publishChan = make(chan *packet.PublishMessage, q.meta.windowSize)

	// read client unRec or unComp packet first
	q.readSessionUnFinishMessage()
	q.listenPublishChan()

	// waiting for exit
	<-ctx.Done()
	if err := q.Close(); err != nil {
		logger.Logger.Warn("QoS1: close error = ", zap.Error(err))
	}
	if err := q.afterClose(); err != nil {
		logger.Logger.Warn("QoS1: after close error = ", zap.Error(err))
	}
}

func (q *QoS2) listenPublishChan() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			if msg.PubReceived {
				q.meta.writer.WritePacket(msg.PubRelPacket)
			} else {
				msg.PublishPacket.QoS = broker.QoS2
				q.meta.writer.WritePacket(msg.PublishPacket)
			}
			q.queue.WritePacket(msg)
			if !msg.FromSession {
				q.meta.latestMessageID = msg.MessageID
			}
		default:
			if err := store.ReadPublishMessage(q.ctx, q.meta.topic, q.meta.latestMessageID, q.meta.windowSize, false, q.writeToPublishChan); err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("store", q.meta.topic))
			}
		}
	}
}

// writeToPublishChan is not concurrent safe,it must be called in a single goroutine
func (q *QoS2) writeToPublishChan(message *packet.PublishMessage) {
	if err := q.Publish(message); err != nil {
		logger.Logger.Warn("write to publishChan error", zap.Error(err), zap.String("topic", q.meta.topic))
	}
}

func (q *QoS2) readSessionUnFinishMessage() {
	for _, msg := range q.session.ReadTopicUnFinishedMessage(q.meta.topic) {
		if msg.PubReceived {
			pubRelPacket := packet.NewPublishRel()
			pubRelPacket.PacketID = util.StringToPacketID(msg.PacketID)
			q.writeToPublishChan(&packet.PublishMessage{
				PubRelPacket: pubRelPacket,
				PubReceived:  true,
				FromSession:  true,
			})
			continue
		}
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		publishMessage, err := store.DefaultMessageStore.ReadTopicMessagesByID(ctx, q.meta.topic, msg.MessageID, 1, true)
		cancel()
		if err != nil {
			logger.Logger.Error("read client.proto unAck publishChan message error", zap.Error(err), zap.String("store", q.meta.topic), zap.String("messageID", msg.MessageID))
			continue
		}
		for _, m := range publishMessage {
			m.FromSession = true
			q.writeToPublishChan(&m)
		}
	}
}

func (q *QoS2) Close() error {
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.cancel()
	return nil
}

func (q *QoS2) HandlePublishRec(pubrec *packets.Pubrec) {
	q.queue.HandlePublishRec(pubrec)
}

func (q *QoS2) HandelPublishComp(pubcomp *packets.Pubcomp) {
	q.queue.HandlePublishComp(pubcomp)
}

func (q *QoS2) afterClose() error {
	q.session.SetTopicLatestPushedMessageID(q.meta.topic, q.meta.latestMessageID)
	q.session.CreateTopicUnFinishedMessage(q.meta.topic, q.queue.getUnFinishPacket())
	return nil
}

func (q *QoS2) Publish(publish *packet.PublishMessage) error {
	if q.meta.subOption.NoLocal == true && publish.ClientID == q.meta.writer.GetID() {
		return nil
	}
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.publishChan <- publish
	return nil
}
