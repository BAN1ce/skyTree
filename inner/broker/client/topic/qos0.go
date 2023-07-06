package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

// PublishListener is the interface of the publish event listener.
// It is used to listen the publish event of the topic.
// The event will be triggered when the topic receives a publishPacket from the client.
// The event will be triggered with two parameters, the first one is the topic name, the second one is the publishPacket.
type PublishListener interface {
	CreatePublishEvent(topic string, handler func(...interface{}))
	DeletePublishEvent(topic string, handler func(i ...interface{}))
}

// QoS0 is a topic with QoS0
type QoS0 struct {
	topic           string
	writer          PublishWriter
	publishListener PublishListener
}

func NewQoS0(topic string, writer PublishWriter, listener PublishListener) *QoS0 {
	return &QoS0{
		topic:           topic,
		writer:          writer,
		publishListener: listener,
	}
}

// Start starts the QoS0 topic, and it will block until the context is done.
func (t *QoS0) Start(ctx context.Context) {
	t.publishListener.CreatePublishEvent(t.topic, t.handler)
	<-ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Warn("QoS0: close error", zap.Error(err))
	}
	t.afterClose()
}

func (t *QoS0) HandlePublishAck(puback *packets.Puback) {
	// do nothing
	return
}

// handler is the handler of the topic, it will be called when the event is triggered.
func (t *QoS0) handler(i ...interface{}) {
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
			logger.Logger.Error("ListenTopicPublishEvent: topic error", zap.String("topic", topic), zap.String("QoS0 topic", t.topic))
			return
		}
		pub := copyPublish(p)
		pub.QoS = pkg.QoS0
		t.writer.WritePacket(pub)
		pool.PublishPool.Put(pub)
	}
}

// publish writes the publishPacket to the writer.
func (t *QoS0) publish(topic string, publish *packets.Publish) {
	var publishPacket = pool.PublishPool.Get()
	defer pool.PublishPool.Put(publishPacket)
	pool.CopyPublish(publishPacket, publish)
	publishPacket.QoS = pkg.QoS0
	if topic != t.topic {
		logger.Logger.Warn("QoS0: topic error", zap.String("topic", topic), zap.String("QoS0 topic", t.topic))
		return
	}
	t.writer.WritePacket(publishPacket)
}

// Close closes the QoS0 topic and remove itself from the event.
func (t *QoS0) Close() error {
	return nil
}
func (t *QoS0) afterClose() {
	t.publishListener.DeletePublishEvent(t.topic, t.handler)
}

func (t *QoS0) HandlePublishRec(pubrec *packets.Pubrec) {
	return
}

func (t *QoS0) HandelPublishComp(pubcomp *packets.Pubcomp) {
	return
}
