package qos0

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

// QoS0 is Topic with QoS0
type QoS0 struct {
	ctx             context.Context
	cancel          context.CancelFunc
	topic           string
	writer          broker.PublishWriter
	publishListener broker.PublishListener
}

func NewQoS0(topic string, writer broker.PublishWriter, listener broker.PublishListener) *QoS0 {
	return &QoS0{
		topic:           topic,
		writer:          writer,
		publishListener: listener,
	}
}

// Start starts the QoS0 Topic, and it will block until the context is done.
// It will create a publish event listener to listen the publish event of the store.
func (t *QoS0) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.publishListener.CreatePublishEvent(t.topic, t.handler)
	<-t.ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Warn("QoS0: close error", zap.Error(err))
	}
	t.afterClose()
}

// handler is the handler of the topic, it will be called when the published packet event is triggered.
func (t *QoS0) handler(i ...interface{}) {
	if t.ctx.Err() != nil {
		logger.Logger.Warn("QoS0: handler error, context canceled", zap.Error(t.ctx.Err()), zap.String("topic", t.topic), zap.String("writer", t.writer.GetID()))
		return
	}
	if len(i) == 2 {
		topic, ok := i[0].(string)
		if !ok {
			logger.Logger.Error("ListenTopicPublishEvent: type error")
			return
		}
		p, ok := i[1].(*packets.Publish)
		if !ok {
			logger.Logger.Error("ListenTopicPublishEvent: type error")
		}
		if topic != t.topic || p.Topic != t.topic {
			logger.Logger.Error("ListenTopicPublishEvent: store error", zap.String("store", topic), zap.String("QoS0 store", t.topic))
			return
		}

		// copy the published packet and set the QoS to QoS0
		var publishPacket = pool.PublishPool.Get()
		pool.CopyPublish(publishPacket, p)
		publishPacket.QoS = broker.QoS0
		t.writer.WritePacket(publishPacket)
		pool.PublishPool.Put(publishPacket)
	}
}

// Close closes the QoS0
func (t *QoS0) Close() error {
	t.cancel()
	return nil
}

// afterClose is the function which will be called after the QoS0 is closed.
// It will delete the publish event listener.
func (t *QoS0) afterClose() {
	t.publishListener.DeletePublishEvent(t.topic, t.handler)
}
