package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/window"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

type StoreEvent interface {
	CreateListenMessageStoreEvent(topic string, handler func(...interface{}))
	DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{}))
}

type QoS1 struct {
	ctx                context.Context
	cancel             context.CancelFunc
	meta               *meta
	windows            *window.Windows
	publishChan        chan *packet.PublishMessage
	publishQueue       *PublishQueue
	lastAckedMessageID string
	writer             PublishWriter
	pkg.ClientMessageStore
	pkg.SessionTopic
	StoreEvent
}

func NewQos1(topic string, windowSize int, store pkg.ClientMessageStore, sessionTopic pkg.SessionTopic, writer PublishWriter, storeEvent StoreEvent) *QoS1 {
	t := &QoS1{
		meta: &meta{
			topic:      topic,
			qos:        pkg.QoS1,
			windowSize: windowSize,
			writer:     writer,
		},
		ClientMessageStore: store,
		SessionTopic:       sessionTopic,
		writer:             writer,
		publishQueue:       NewPublishQueue(writer),
		StoreEvent:         storeEvent,
	}
	return t
}

func (t *QoS1) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	if t.meta.windowSize == 0 {
		t.meta.windowSize = config.GetTopic().WindowSize
	}
	t.windows = window.NewWindows(ctx, t.meta.windowSize)
	t.publishChan = make(chan *packet.PublishMessage, t.meta.windowSize)
	// read session unAck publishChan first
	t.readSessionUnAck()
	t.pushMessage()
	// waiting for exit, prevent pushMessage use goroutine
	<-ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Warn("QoS1: close error = ", zap.Error(err))
	}
	if err := t.afterClose(); err != nil {
		logger.Logger.Warn("QoS1: after close error = ", zap.Error(err))
	}
}

func (t *QoS1) readSessionUnAck() {
	messageID := t.SessionTopic.ReadTopicUnAckMessageID(t.meta.topic)
	for _, id := range messageID {
		msg, err := t.ClientMessageStore.ReadTopicMessagesByID(context.TODO(), t.meta.topic, id, 1, true)
		if err != nil {
			logger.Logger.Error("read session unAck publishChan message error", zap.Error(err), zap.String("topic", t.meta.topic), zap.String("messageID", id))
			continue
		}
		for _, m := range msg {
			t.publishChan <- &m
		}
	}
}

func (t *QoS1) HandlePublishAck(puback *packets.Puback) {
	if t.publishQueue.HandlePublishAck(puback) {
		t.windows.Put()
	}
}

func (t *QoS1) pushMessage() {
	var f func(i ...interface{})
	defer t.StoreEvent.DeleteListenMessageStoreEvent(t.meta.topic, f)
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg, ok := <-t.publishChan:
			if !ok {
				return
			}
			t.windows.Get()
			msg.Packet.QoS = pkg.QoS1
			t.writer.WritePacket(msg.Packet)
			t.publishQueue.WritePacket(msg)
		default:
			var (
				// FIXME: context timeout config
				ctx, cancel = context.WithTimeout(t.ctx, 10*time.Second)
			)
			if t.lastAckedMessageID != "" {
				if t.readStore(context.TODO(), t.lastAckedMessageID, false) > 0 {
					cancel()
					continue
				}
			}
			f = func(i ...interface{}) {
				if len(i) != 2 {
					return
				}
				topic, _ := i[0].(string)
				id, _ := i[1].(string)
				if topic != t.meta.topic {
					logger.Logger.Warn("topic not match", zap.String("topic", topic), zap.String("expect", t.meta.topic))
					return
				}
				t.readStore(ctx, id, true)
				cancel()
			}
			t.StoreEvent.CreateListenMessageStoreEvent(t.meta.topic, f)
			<-ctx.Done()
			t.StoreEvent.DeleteListenMessageStoreEvent(t.meta.topic, f)
		}
	}
}

func (t *QoS1) readStore(ctx context.Context, startMessageID string, include bool) int {
	var (
		read         int
		message, err = t.ClientMessageStore.ReadTopicMessagesByID(ctx, t.meta.topic, startMessageID, t.meta.windowSize, include)
	)
	if err != nil {
		logger.Logger.Error("read topic message error", zap.Error(err), zap.String("topic", t.meta.topic), zap.String("messageID", startMessageID))
		return 0
	}
	for _, m := range message {
		select {
		case t.publishChan <- &m:
			read++
			t.lastAckedMessageID = m.MessageID
		default:
		}
	}
	return read
}

func (t *QoS1) Close() error {
	t.cancel()
	return nil
}

func (t *QoS1) afterClose() error {
	// save unAck messageID to session when exit
	t.SessionTopic.SaveTopicUnAckMessageID(t.meta.topic, t.publishQueue.GetUnAckMessageID())
	return t.publishQueue.Close()
}
