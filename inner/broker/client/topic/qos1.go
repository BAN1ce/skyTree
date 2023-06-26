package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/window"
)

type QoS1 struct {
	ctx           context.Context
	cancel        context.CancelFunc
	meta          *meta
	windows       *window.Windows
	publishChan   chan packet.Publish
	publishQueue  *PublishQueue
	lastMessageID string
	pkg.ClientMessageStore
	pkg.SessionTopic
}

func NewQos1(topic string, windowSize int, store pkg.ClientMessageStore, sessionTopic pkg.SessionTopic, writer PublishWriter) *QoS1 {
	t := &QoS1{
		meta: &meta{
			topic:      topic,
			qos:        pkg.QoS1,
			windowSize: windowSize,
			writer:     writer,
		},
		ClientMessageStore: store,
		SessionTopic:       sessionTopic,
		publishQueue:       NewPublishQueue(writer),
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
	t.readMessage(t.ctx)
	// waiting for exit, prevent readMessage use goroutine
	<-ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Error("QoS1: close error = ", err)
	}
}

func (t *QoS1) readSessionUnAck() {
	var topics = t.SessionTopic.ReadSubTopics()
	for topic := range topics {
		messageID := t.SessionTopic.ReadTopicUnAckMessageID(topic)
		for _, id := range messageID {
			msg := t.ClientMessageStore.ReadTopicMessagesByID(context.TODO(), topic, id, 1, true)
			if len(msg) == 0 {
				continue
			}
			t.publishChan <- msg[0]
		}
	}
}
func (t *QoS1) readMessage(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-t.publishChan:
			t.windows.Get()
			t.publishQueue.WritePacket(msg)
		default:
			if t.readStoreToFillChannel() > 0 {
				continue
			}
			// waiting for store event
			<-t.storeReady().Done()
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

func (t *QoS1) storeReady() context.Context {
	var (
		ctx, cancel = context.WithCancel(t.ctx)
	)
	// TODO: should delete listener when exit
	t.OnceListenTopicStoreEvent(t.ctx, t.meta.topic, func(topic, id string) {
		if topic != t.meta.topic {
			logger.Logger.Warn("topic not match", "topic", topic, "expect", t.meta.topic)
			return
		}
		t.readStore(ctx, id, true)
		cancel()
	})
	return ctx
}

func (t *QoS1) Close() error {
	t.cancel()
	// save unAck messageID to session when exit
	t.SessionTopic.CreateTopicUnAckMessageID(t.meta.topic, t.publishQueue.GetUnAckMessageID())

	return nil
}

func (t *QoS1) Release() error {
	t.cancel()
	t.SessionTopic.ReleaseTopicSession(t.meta.topic)
	return nil
}

func (t *QoS1) GetQoS() pkg.QoS {
	return pkg.QoS1
}
