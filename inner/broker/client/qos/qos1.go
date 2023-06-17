package qos

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/eclipse/paho.golang/packets"
)

type Writer interface {
	WritePacket(packet packets.Packet)
}

type QoS1 struct {
	ctx       context.Context
	cancel    context.CancelFunc
	topic     string
	message   chan pkg.Message
	bucket    chan struct{}
	qos1Queue QoS1Queue
	// from outside
	windowSize int
	store      pkg.ClientMessageStore
	pkg.SessionTopic
	writer Writer
}

func NewQos1(topic string, windowSize int, store pkg.ClientMessageStore, sessionTopic pkg.SessionTopic, writer Writer) *QoS1 {
	t := &QoS1{
		topic:        topic,
		windowSize:   0,
		store:        store,
		SessionTopic: sessionTopic,
		writer:       writer,
	}
	return t
}

func (t *QoS1) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	if t.windowSize == 0 {
		t.windowSize = config.GetTopic().WindowSize
	}
	t.message = make(chan pkg.Message, t.windowSize)
	t.bucket = make(chan struct{}, t.windowSize)
	// FIXME
	// t.qos1Queue = NewQoS1Queue(t.writeMessage, t.windowSize)
	t.readSessionUnAck()
	t.readMessage(t.ctx)
}

func (t *QoS1) readSessionUnAck() {
	var topics = t.SessionTopic.ReadSubTopics()
	for topic := range topics {
		messageID := t.SessionTopic.ReadTopicUnAckMessageID(topic)
		for _, id := range messageID {
			msg := t.store.ReadTopicMessageByID(context.TODO(), topic, id, 1)
			if len(msg) == 0 {
				continue
			}
			t.message <- msg[0]
		}
	}
}
func (t *QoS1) readMessage(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-t.message:
			t.writeMessage(msg)
		default:
			if t.readStoreToFillChannel() {
				continue
			}
			// waiting for store event
			<-t.storeReady().Done()
		}
	}
}

// readStoreToFillChannel read message from store util fill the messages channel or all topics done
func (t *QoS1) readStoreToFillChannel() bool {
	var (
		ctx, cancel = context.WithTimeout(context.TODO(), config.GetStore().ReadTimeout)
		filled      bool
	)
	defer cancel()
	// TODO: 继续改造client 保证一个topic一个goroutine一个环境，互相不干扰。
	lastMessageID, ok := t.SessionTopic.ReadTopicLastAckedMessageID(t.topic)
	if !ok {
		logger.Logger.Info("session last acked message id not found", "topic", t.topic)
	}
	for _, m := range t.store.ReadTopicMessageAfterID(ctx, t.topic, lastMessageID, t.windowSize) {
		select {
		case t.message <- m:
			filled = true
		default:
		}
	}
	return filled
}

func (t *QoS1) storeReady() context.Context {
	var (
		ctx, cancel = context.WithCancel(t.ctx)
	)
	t.OnceListenPublishEvent(t.topic, func(topic, id string) {
		if t.readMessageStoreByID(topic, id) > 0 {
			cancel()
			return
		}
		logger.Logger.Warn("readMessageStoreByID: no message", "topic = ", topic, "id = ", id)
		cancel()
	})
	return ctx
}

func (t *QoS1) writeMessage(msg pkg.Message) {
	if msg.GetQoS() == pkg.QoS0 {
		t.writer.WritePacket(msg.Decode())
	} else {
		<-t.bucket
		t.writer.WritePacket(msg.Decode())
	}
}

func (t *QoS1) Close() error {
	t.cancel()
	// save unAcked message id to session
	t.SessionTopic.CreateTopicUnAckMessageID(t.topic, t.qos1Queue.UnAckMessageID())
	return nil
}

func (t *QoS1) Release() error {
	t.cancel()
	t.SessionTopic.ReleaseTopicSession(t.topic)
	return nil
}

func (t *QoS1) readMessageStoreByID(topic, id string) int {
	var (
		i           int
		ctx, cancel = context.WithTimeout(t.ctx, config.GetStore().ReadTimeout)
	)
	defer cancel()
	if message := t.store.ReadTopicMessageByID(ctx, topic, id, t.windowSize); len(message) > 0 {
		for _, msg := range message {
			select {
			case t.message <- msg:
				i++
			default:
			}
		}
	}
	return i
}

func (t *QoS1) GetQoS() pkg.QoS {
	return pkg.QoS1
}
