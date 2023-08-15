package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type StoreEvent interface {
	CreateListenMessageStoreEvent(topic string, handler func(...interface{}))
	DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{}))
}

type QoS1 struct {
	ctx          context.Context
	cancel       context.CancelFunc
	meta         *meta
	publishChan  chan *packet.PublishMessage
	publishQueue *PublishQueue
	session      QoS1Session
	*StoreHelp
}

func NewQos1(topic string, writer PublishWriter, help *StoreHelp, session QoS1Session) *QoS1 {
	latestMessageID, _ := session.ReadTopicLatestPushedMessageID(topic)
	t := &QoS1{
		meta: &meta{
			topic:           topic,
			qos:             broker.QoS1,
			writer:          writer,
			latestMessageID: latestMessageID,
		},
		publishQueue: NewPublishQueue(writer),
		StoreHelp:    help,
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
	// read client.proto unAck publishChan first
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
	for _, id := range q.session.ReadTopicUnAckMessageID(q.meta.topic) {
		msg, err := q.ClientMessageStore.ReadTopicMessagesByID(context.TODO(), q.meta.topic, id, 1, true)
		if err != nil {
			logger.Logger.Error("read client.proto unAck publishChan message error", zap.Error(err), zap.String("store", q.meta.topic), zap.String("messageID", id))
			continue
		}
		for _, m := range msg {
			q.publishChan <- &m
		}
	}
}

func (q *QoS1) HandlePublishAck(puback *packets.Puback) {
	if !q.publishQueue.HandlePublishAck(puback) {
		logger.Logger.Warn("QoS1: handle publish ack failed, packetID not found", zap.Uint16("packetID", puback.PacketID), zap.String("store", q.meta.topic))
	}
}

func (q *QoS1) pushMessage() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			msg.Packet.QoS = broker.QoS1
			q.meta.writer.WritePacket(msg.Packet)
			q.publishQueue.WritePacket(msg)
		default:
			if err := q.StoreHelp.readStore(q.ctx, q.meta.topic, q.meta.latestMessageID, q.meta.windowSize, false, q.writeToPublishChan); err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("store", q.meta.topic))
			}
		}
	}
}

func (q *QoS1) writeToPublishChan(message *packet.PublishMessage) {
	select {
	case q.publishChan <- message:
		q.meta.latestMessageID = message.MessageID
	default:

	}
}

func (q *QoS1) Close() error {
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.cancel()
	return nil
}

// afterClose save unAck messageID to client.proto when exit
// should be call after close and only call once
func (q *QoS1) afterClose() error {
	if err := q.publishQueue.Close(); err != nil {
		return err
	}
	q.session.CreateTopicUnAckMessageID(q.meta.topic, q.publishQueue.GetUnAckMessageID())
	q.session.SetTopicLatestPushedMessageID(q.meta.topic, q.meta.latestMessageID)
	return nil
}

func (q *QoS1) GetUnAckedMessageID() []string {
	return q.publishQueue.GetUnAckMessageID()
}
