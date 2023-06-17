package qos

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"io"
)

type PublishEvent interface {
	ListenPublishEvent(topic string, handler func(topic string, publish *packets.Publish)) func(i ...interface{})
	RemovePublishEvent(topic string, listener func(i ...interface{}))
}

type QoS0 struct {
	topic    string
	writer   io.Writer
	listener func(i ...interface{})
}

func NewQoS0(topic string, writer io.Writer) *QoS0 {
	return &QoS0{
		topic:  topic,
		writer: writer,
	}
}

func (t *QoS0) Start(ctx context.Context) {
	t.listener = event.ListenTopicPublishEvent(t.topic, func(topic string, publish *packets.Publish) {
		t.publish(topic, publish)
	})
}

func (t *QoS0) publish(topic string, publish *packets.Publish) {
	if topic != t.topic {
		logger.Logger.Error("QoS0: topic error", topic, t.topic)
		return
	}
	if _, err := publish.WriteTo(t.writer); err != nil {
		logger.Logger.Info("QoS0: write to client error = ", err)
	}
}

func (t *QoS0) Close() error {
	event.RemoveTopicPublishEvent(t.topic, t.listener)
	return nil
}
