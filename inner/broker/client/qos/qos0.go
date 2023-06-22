package qos

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"io"
)

type TopicPublishListener interface {
	CreatePublishEvent(topic string, handler func(...interface{}))
	DeletePublishEvent(topic string, handler func(i ...interface{}))
}

type QoS0 struct {
	topic           string
	writer          io.Writer
	publishListener TopicPublishListener
}

func NewQoS0(topic string, writer io.Writer, listener TopicPublishListener) *QoS0 {
	return &QoS0{
		topic:           topic,
		writer:          writer,
		publishListener: listener,
	}
}

func (t *QoS0) Start(ctx context.Context) {
	t.publishListener.CreatePublishEvent(t.topic, t.handler)
	<-ctx.Done()
	if err := t.Close(); err != nil {
		logger.Logger.Error("QoS0: close error = ", err)
	}
}

func (t *QoS0) handler(i ...interface{}) {
	if len(i) == 2 {
		topic, ok := i[0].(string)
		if !ok {
			logger.Logger.Error("ListenTopicPublishEvent: type error")
			return
		}
		p, ok := i[1].(*packets.Publish)
		if ok {
			t.publish(topic, p)
		} else {
			logger.Logger.Error("ListenTopicPublishEvent: type error")
		}
	}
}

func (t *QoS0) publish(topic string, publish *packets.Publish) {
	if topic != t.topic {
		logger.Logger.Error("QoS0: topic error", topic, t.topic)
		return
	}
	if _, err := publish.WriteTo(t.writer); err != nil {
		logger.Logger.Warn("QoS0: write to client error = ", err)
	}
}

func (t *QoS0) Close() error {
	t.publishListener.DeletePublishEvent(t.topic, t.handler)
	return nil
}
