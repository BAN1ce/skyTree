package topic

import (
	"context"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/qos0"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/qos1"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/qos2"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type Topic interface {
	Start(ctx context.Context)
	Close() error
	Publish(publish *packet.PublishMessage) error
}
type QoS1Handle interface {
	HandlePublishAck(pubAck *packets.Puback)
}

type QoS2Handle interface {
	HandlePublishRec(pubRec *packets.Pubrec)
	HandelPublishComp(pubComp *packets.Pubcomp)
}

type Option func(topics *Topics)

func WithWriter(writer broker.PublishWriter) Option {
	return func(topic *Topics) {
		topic.writer = writer
	}
}

func WithWindowSize(size int) Option {
	return func(topic *Topics) {
		topic.windowSize = size
	}
}

type Topics struct {
	ctx        context.Context
	topic      map[string]Topic
	session    broker.SessionTopic
	writer     broker.PublishWriter
	windowSize int
}

func NewTopics(ctx context.Context, ops ...Option) *Topics {
	t := &Topics{
		ctx:   ctx,
		topic: make(map[string]Topic),
	}
	for _, op := range ops {
		op(t)
	}
	return t
}

func NewTopicWithSession(ctx context.Context, session broker.SessionTopic, op ...Option) *Topics {
	t := NewTopics(ctx, op...)
	t.session = session
	for topic, subOption := range session.ReadSubTopics() {
		logger.Logger.Debug("read store from client.proto = ", zap.String("store", topic), zap.Int32("qos", subOption.QoS))
		t.CreateTopic(topic, subOption)

	}
	return t
}

func (t *Topics) CreateTopic(topicName string, option *proto.SubOption) {
	var (
		topic Topic
	)
	if t, ok := t.topic[topicName]; ok {
		if err := t.Close(); err != nil {
			logger.Logger.Warn("close store error = ", zap.Error(err))
		}
	}
	switch broker.Int32ToQoS(option.QoS) {
	case broker.QoS0:
		logger.Logger.Debug("create store with QoS0", zap.String("topic", topicName))
		topic = t.createQoS0Topic(topicName, option)
	case broker.QoS1:
		logger.Logger.Debug("create store with QoS1", zap.String("topic", topicName))
		topic = t.createQoS1Topic(topicName, option)
	case broker.QoS2:
		logger.Logger.Debug("create store with QoS2", zap.String("topic", topicName))
		topic = t.createQoS2Topic(topicName, option)
	default:
		logger.Logger.Warn("create store with wrong QoS ", zap.Int32("qos", option.QoS))
		return
	}
	t.session.CreateSubTopic(topicName, option)
	t.topic[topicName] = topic
	go topic.Start(t.ctx)
}

func (t *Topics) HandlePublishAck(topic string, puback *packets.Puback) {
	if topic, ok := t.topic[topic]; ok {
		if t, ok := topic.(QoS1Handle); ok {
			t.HandlePublishAck(puback)
			return
		}
	}
	logger.Logger.Warn("handle publish Ack failed, maybe topic not exists or handle type error")
}

func (t *Topics) HandlePublishRec(topic string, pubrec *packets.Pubrec) {
	if topic, ok := t.topic[topic]; ok {
		if t, ok := topic.(QoS2Handle); ok {
			t.HandlePublishRec(pubrec)
			return
		}
		logger.Logger.Warn("handle publish Rec failed, handle type error not QoS2")
		return
	}
	logger.Logger.Warn("handle publish Rec failed, topic not exists")
}

func (t *Topics) HandelPublishComp(topic string, pubcomp *packets.Pubcomp) {
	if topic, ok := t.topic[topic]; ok {
		if t, ok := topic.(QoS2Handle); ok {
			t.HandelPublishComp(pubcomp)
			return
		}
		logger.Logger.Warn("handle publish Comp failed, handle type error not QoS2")
		return

	}
	logger.Logger.Warn("handle publish Comp failed, topic not exists ")
}

func (t *Topics) createQoS0Topic(topicName string, option *proto.SubOption) Topic {
	return qos0.NewQoS0(topicName, qos0.WithPublishWriter(t.writer), qos0.WithPublishListener(event.GlobalEvent), qos0.WithSubOption(option))
}

func (t *Topics) createQoS1Topic(topicName string, option *proto.SubOption) Topic {
	return qos1.NewQos1(topicName, qos1.WithSession(t.session), qos1.WithPublishWriter(t.writer), qos1.WithSubOption(option))
}

func (t *Topics) createQoS2Topic(topicName string, option *proto.SubOption) Topic {
	return qos2.NewQos2(topicName, qos2.WithSession(t.session), qos2.WithPublishWriter(t.writer), qos2.WithSubOption(option))
}

func (t *Topics) DeleteTopic(topicName string) {
	if _, ok := t.topic[topicName]; ok {
		if err := t.topic[topicName].Close(); err != nil {
			logger.Logger.Warn("topics close store failed", zap.Error(err), zap.String("store", topicName))
		}
		delete(t.topic, topicName)
	} else {
		logger.Logger.Warn("topics delete topic, topic not exists", zap.String("topic", topicName))
	}
}

func (t *Topics) Close() error {
	// for no sub client
	if t == nil {
		return nil
	}
	for topicName, topic := range t.topic {
		if err := topic.Close(); err != nil {
			logger.Logger.Error("close topic error", zap.Error(err), zap.String("topic", topicName))
		}
		t.DeleteTopic(topicName)
	}
	return nil
}
func (t *Topics) Publish(topic string, message *packet.PublishMessage) error {
	if _, ok := t.topic[topic]; !ok {
		return errs.ErrTopicNotExistsInSubTopics
	}
	return t.topic[topic].Publish(message)
}
