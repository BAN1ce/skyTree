package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/window"
	"github.com/eclipse/paho.golang/packets"
	"time"
)

type StoreEvent interface {
	CreateListenMessageStoreEvent(topic string, handler func(...interface{}))
	DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{}))
}

type QoS1 struct {
	ctx           context.Context
	cancel        context.CancelFunc
	meta          *meta
	windows       *window.Windows
	publishChan   chan packet.Publish
	publishQueue  *PublishQueue
	lastMessageID string
	writer        PublishWriter
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
	t.publishChan = make(chan packet.Publish, t.meta.windowSize)
	// read session unAck publishChan first
	t.readSessionUnAck()
	t.pushMessage()
	// waiting for exit, prevent pushMessage use goroutine
	<-ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Error("QoS1: close error = ", err)
	}
}

func (t *QoS1) readSessionUnAck() {
	messageID := t.SessionTopic.ReadTopicUnAckMessageID(t.meta.topic)
	for _, id := range messageID {
		msg := t.ClientMessageStore.ReadTopicMessagesByID(context.TODO(), t.meta.topic, id, 1, true)
		if len(msg) == 0 {
			logger.Logger.Warn("read session unAck publishChan id not found", "topic", t.meta.topic, "messageID", id)
			continue
		}
		t.publishChan <- msg[0]
	}
}

func (t *QoS1) HandlePublishAck(puback *packets.Puback) {
	if t.publishQueue.HandlePublishAck(puback){
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
		case msg := <-t.publishChan:
			t.windows.Get()
			t.writer.WritePacket(msg.Packet)
			t.publishQueue.WritePacket(msg)
		default:
			if t.readStoreToFillChannel() > 0 {
				continue
			}
			var (
				ctx, cancel = context.WithTimeout(t.ctx, 10*time.Second)
			)
			f = func(i ...interface{}) {
				if len(i) != 2 {
					return
				}
				topic, _ := i[0].(string)
				id, _ := i[1].(string)
				if topic != t.meta.topic {
					logger.Logger.Warn("topic not match", "topic", topic, "expect", t.meta.topic)
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

// readStoreToFillChannel read publishChan from store util fill the messages channel or all topics done
func (t *QoS1) readStoreToFillChannel() int {
	var (
		ctx, cancel = context.WithTimeout(context.TODO(), config.GetStore().ReadTimeout)
	)
	defer cancel()
	lastAckMessageID, ok := t.SessionTopic.ReadTopicLastAckedMessageID(t.meta.topic)
	// first time subscribe topic or session not acked publishChan id
	if !ok {
		logger.Logger.Info("session last acked publishChan id not found,maybe first time subscribe", "topic", t.meta.topic)
		return 0
	}
	return t.readStore(ctx, lastAckMessageID, false)
}

func (t *QoS1) readStore(ctx context.Context, startMessageID string, include bool) int {
	var read int
	for _, m := range t.ClientMessageStore.ReadTopicMessagesByID(ctx, t.meta.topic, startMessageID, t.meta.windowSize, include) {
		select {
		case t.publishChan <- m:
			read++
		default:
		}
	}
	return read
}

func (t *QoS1) Close() error {
	t.cancel()
	// save unAck messageID to session when exit
	t.SessionTopic.CreateTopicUnAckMessageID(t.meta.topic, t.publishQueue.GetUnAckMessageID())
	return t.publishQueue.Close()
}

func (t *QoS1) GetQoS() pkg.QoS {
	return pkg.QoS1
}
