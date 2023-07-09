package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type StoreEvent interface {
	CreateListenMessageStoreEvent(topic string, handler func(...interface{}))
	DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{}))
}

type QoS1MessageSession interface {
	SaveTopicUnAckMessageID(topic string, messageID []string)
	ReadTopicUnAckMessageID(topic string) []string
}

type QoS1 struct {
	ctx                context.Context
	cancel             context.CancelFunc
	meta               *meta
	publishChan        chan *packet.PublishMessage
	publishQueue       *PublishQueue
	lastAckedMessageID string
	writer             PublishWriter
	QoS1MessageSession
	*StoreHelp
}

func NewQos1(topic string, unAckSession QoS1MessageSession, writer PublishWriter, help *StoreHelp) *QoS1 {
	t := &QoS1{
		meta: &meta{
			topic:  topic,
			qos:    pkg.QoS1,
			writer: writer,
		},
		QoS1MessageSession: unAckSession,
		writer:             writer,
		publishQueue:       NewPublishQueue(writer),
		StoreHelp:          help,
	}
	return t
}

func (q *QoS1) Start(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
	if q.meta.windowSize == 0 {
		// FIXME: config.GetTopic().WindowSize,use client or another config
		q.meta.windowSize = config.GetTopic().WindowSize
	}
	q.publishChan = make(chan *packet.PublishMessage, q.meta.windowSize)
	// read session unAck publishChan first
	q.readSessionUnAck()
	q.pushMessage()
	// waiting for exit, prevent pushMessage use goroutine
	<-ctx.Done()
	if err := q.Close(); err != nil {
		logger.Logger.Warn("QoS1: close error = ", zap.Error(err))
	}
	if err := q.afterClose(); err != nil {
		logger.Logger.Warn("QoS1: after close error = ", zap.Error(err))
	}
}

func (q *QoS1) readSessionUnAck() {
	messageID := q.QoS1MessageSession.ReadTopicUnAckMessageID(q.meta.topic)
	for _, id := range messageID {
		msg, err := q.ClientMessageStore.ReadTopicMessagesByID(context.TODO(), q.meta.topic, id, 1, true)
		if err != nil {
			logger.Logger.Error("read session unAck publishChan message error", zap.Error(err), zap.String("topic", q.meta.topic), zap.String("messageID", id))
			continue
		}
		for _, m := range msg {
			q.publishChan <- &m
		}
	}
}

func (q *QoS1) HandlePublishAck(puback *packets.Puback) {
	if !q.publishQueue.HandlePublishAck(puback) {
		logger.Logger.Warn("QoS1: handle publish ack failed, packetID not found", zap.Uint16("packetID", puback.PacketID), zap.String("topic", q.meta.topic))
	}
}

func (q *QoS1) pushMessage() {
	var f func(i ...interface{})
	defer q.StoreEvent.DeleteListenMessageStoreEvent(q.meta.topic, f)
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			msg.Packet.QoS = pkg.QoS1
			q.writer.WritePacket(msg.Packet)
			q.publishQueue.WritePacket(msg)
		default:
			if err := q.StoreHelp.readStore(q.ctx, q.publishChan, q.meta.topic, q.meta.windowSize, false); err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("topic", q.meta.topic))
			}
		}
	}
}

func (q *QoS1) Close() error {
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.cancel()
	return nil
}

// afterClose save unAck messageID to session when exit
// should be call after close and only call once
func (q *QoS1) afterClose() error {
	// save unAck messageID to session when exit
	q.QoS1MessageSession.SaveTopicUnAckMessageID(q.meta.topic, q.publishQueue.GetUnAckMessageID())
	return q.publishQueue.Close()
}

func (q *QoS1) HandlePublishRec(pubrec *packets.Pubrec) {
	// do nothing
	return
}

func (q *QoS1) HandelPublishComp(pubcomp *packets.Pubcomp) {
	// do nothing
	return
}
