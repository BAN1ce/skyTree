package qos1

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/store"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
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
	session      Session
	*store.Help
}

func NewQos1(topic string, writer broker.PublishWriter, help *store.Help, session Session) *QoS1 {
	latestMessageID, _ := session.ReadTopicLatestPushedMessageID(topic)
	t := &QoS1{
		meta: &meta{
			topic:           topic,
			qos:             broker.QoS1,
			writer:          writer,
			latestMessageID: latestMessageID,
		},
		publishQueue: NewPublishQueue(writer),
		Help:         help,
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
	q.readSessionUnFinishMessage()
	q.listenPublishChan()
	// waiting for exit, prevent listenPublishChan use goroutine
	<-ctx.Done()
	if err := q.Close(); err != nil {
		logger.Logger.Warn("QoS1: close error = ", zap.Error(err))
	}
	if err := q.afterClose(); err != nil {
		logger.Logger.Warn("QoS1: after close error = ", zap.Error(err))
	}
}

func (q *QoS1) readSessionUnFinishMessage() {
	for _, msg := range q.session.ReadTopicUnFinishedMessage(q.meta.topic) {
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		publishMessage, err := q.ClientMessageStore.ReadTopicMessagesByID(ctx, q.meta.topic, msg.MessageID, 1, true)
		cancel()
		if err != nil {
			logger.Logger.Error("read client.proto unAck publishChan message error", zap.Error(err), zap.String("store", q.meta.topic), zap.String("messageID", msg.MessageID))
			continue
		}
		for _, m := range publishMessage {
			m.FromSession = true
			q.publishChan <- &m
		}
	}
}

func (q *QoS1) HandlePublishAck(pubAck *packets.Puback) {
	if !q.publishQueue.HandlePublishAck(pubAck) {
		logger.Logger.Warn("QoS1: handle publish ack failed, packetID not found", zap.Uint16("packetID", pubAck.PacketID), zap.String("store", q.meta.topic))
	}
}

func (q *QoS1) listenPublishChan() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			msg.PublishPacket.QoS = broker.QoS1
			q.meta.writer.WritePacket(msg.PublishPacket)
			q.publishQueue.WritePacket(msg)
			if !msg.FromSession {
				q.meta.latestMessageID = msg.MessageID
			}
		default:
			if err := q.Help.ReadStore(q.ctx, q.meta.topic, q.meta.latestMessageID, q.meta.windowSize, false, q.writeToPublishChan); err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("store", q.meta.topic))
			}
		}
	}
}

func (q *QoS1) writeToPublishChan(message *packet.PublishMessage) {
	if q.ctx.Err() != nil {
		return
	}
	select {
	case q.publishChan <- message:
	case <-q.ctx.Done():
		close(q.publishChan)
		return
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
	q.session.CreateTopicUnFinishedMessage(q.meta.topic, q.publishQueue.getUnFinishedMessageID())
	q.session.SetTopicLatestPushedMessageID(q.meta.topic, q.meta.latestMessageID)
	return nil
}
