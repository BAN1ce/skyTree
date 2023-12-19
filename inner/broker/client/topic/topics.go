package topic

import (
	"context"
	"fmt"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/inner/broker/message_source"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type Option func(topics *Manager)

type SessionTopicData struct {
	SubOption  *proto.SubOption
	UnFinished []*packet.Message
}

type Manager struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	topic  map[string]topic.Topic
}

func NewManager(ctx context.Context, ops ...Option) *Manager {
	t := &Manager{
		topic: make(map[string]topic.Topic),
	}
	t.ctx, t.cancel = context.WithCancelCause(ctx)
	for _, op := range ops {
		op(t)
	}
	return t
}

func (t *Manager) HandlePublishAck(topicName string, puback *packets.Puback) {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.TopicQoS1); ok {
			t.HandlePublishAck(puback)
			return
		}
	}
	logger.Logger.Warn("handle publish Ack failed, maybe topic not exists or handle type error")
}

func (t *Manager) HandlePublishRec(topicName string, pubrec *packets.Pubrec) {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.TopicQoS2); ok {
			t.HandlePublishRec(pubrec)
			return
		}
		logger.Logger.Warn("handle publish Rec failed, handle type error not QoS2")
		return
	}
	logger.Logger.Warn("handle publish Rec failed, topic not exists")
}

func (t *Manager) HandelPublishComp(topicName string, pubcomp *packets.Pubcomp) {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.TopicQoS2); ok {
			t.HandlePublishComp(pubcomp)
			return
		}
		logger.Logger.Warn("handle publish Comp failed, handle type error not QoS2")
		return

	}
	logger.Logger.Warn("handle publish Comp failed, topic not exists ")
}

func (t *Manager) AddTopic(topicName string, topic topic.Topic) {
	if existTopic, ok := t.topic[topicName]; ok {
		if err := existTopic.Close(); err != nil {
			logger.Logger.Warn("topics close store failed", zap.Error(err), zap.String("store", topicName))
		}
	}
	t.topic[topicName] = topic
	go func() {
		if err := topic.Start(t.ctx); err != nil {
			logger.Logger.Warn("start topic failed", zap.Error(err))
		}
	}()
}

func (t *Manager) DeleteTopic(topicName string) {
	if _, ok := t.topic[topicName]; ok {
		if err := t.topic[topicName].Close(); err != nil {
			logger.Logger.Warn("topics close store failed", zap.Error(err), zap.String("store", topicName))
		}
		delete(t.topic, topicName)
	} else {
		logger.Logger.Warn("topics delete topic, topic not exists", zap.String("topic", topicName))
	}
}

func (t *Manager) Close() error {
	// for no sub client
	if t == nil {
		return nil
	}
	t.cancel(fmt.Errorf("topics close"))
	return nil
}

func (t *Manager) GetUnfinishedMessage() map[string][]*packet.Message {
	var unfinishedMessage = make(map[string][]*packet.Message)
	for topic, topicInstance := range t.topic {
		unfinishedMessage[topic] = topicInstance.GetUnFinishedMessage()
	}
	return unfinishedMessage
}

func (t *Manager) Publish(topic string, message *packet.Message) error {
	if _, ok := t.topic[topic]; !ok {
		return errs.ErrTopicNotExistsInSubTopics
	}
	return t.topic[topic].Publish(message)
}

func CreateTopic(client broker.PacketWriter, options *packets.SubOptions) topic.Topic {
	switch options.QoS {
	case broker.QoS0:
		return NewQoS0(options, client, message_source.NewEventSource(options.Topic, event.GlobalEvent))
	case broker.QoS1:
		return NewQoS1(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	case broker.QoS2:
		return NewQoS2(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	default:
		logger.Logger.Error("create topic failed, qos not support", zap.String("topic", options.Topic), zap.Uint8("qos", options.QoS))
		return nil
	}
}

func CreateTopicFromSession(client broker.PacketWriter, options *packets.SubOptions, unfinished []*packet.Message, latestMessageID string) topic.Topic {
	switch options.QoS {
	case broker.QoS0:
		return NewQoS0(options, client, message_source.NewEventSource(options.Topic, event.GlobalEvent))
	case broker.QoS1:
		return NewQoS1(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, QoS1WithLatestMessageID(latestMessageID))
	case broker.QoS2:
		return NewQoS2(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, QoS2WithLatestMessageID(latestMessageID))
	default:
		logger.Logger.Error("create topic failed, qos not support", zap.String("topic", options.Topic), zap.Uint8("qos", options.QoS))
		return nil
	}

}
