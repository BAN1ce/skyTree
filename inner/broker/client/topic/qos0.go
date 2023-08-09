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
// It is used to listen the publish event from broker.
// The publish event will be triggered when the client publish a message to the broker.
type PublishListener interface {
	CreatePublishEvent(topic string, handler func(i ...interface{}))
	DeletePublishEvent(topic string, handler func(i ...interface{}))
}

// QoS0 is Topic with QoS0
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

// Start starts the QoS0 Topic, and it will block until the context is done.
// It will create a publish event listener to listen the publish event of the store.
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
			logger.Logger.Error("ListenTopicPublishEvent: store error", zap.String("store", topic), zap.String("QoS0 store", t.topic))
			return
		}

		var publishPacket = pool.PublishPool.Get()
		pub := copyPublish(publishPacket, p)
		pub.QoS = pkg.QoS0
		t.writer.WritePacket(pub)
		pool.PublishPool.Put(pub)
	}
}

// Close closes the QoS0
func (t *QoS0) Close() error {
	return nil
}

// afterClose is the function which will be called after the QoS0 is closed.
// It will delete the publish event listener.
func (t *QoS0) afterClose() {
	t.publishListener.DeletePublishEvent(t.topic, t.handler)
}

func (t *QoS0) HandlePublishRec(pubrec *packets.Pubrec) {
	// do nothing
}

func (t *QoS0) HandelPublishComp(pubcomp *packets.Pubcomp) {
	// do nothing
}
