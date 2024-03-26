package topic

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/inner/broker/message_source"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/client"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type Option func(topics *Manager)

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

func (t *Manager) HandlePublishAck(topicName string, puback *packets.Puback) error {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.QoS1); ok {
			t.HandlePublishAck(puback)
			return nil
		} else {
			logger.Logger.Error("handle publish Ack failed, handle type error not QoS1")
			return errs.ErrTopicQoSNotSupport
		}
	}
	logger.Logger.Error("handle publish Ack failed, maybe topic not exists or handle type error")
	return errs.ErrTopicNotExistsInSubTopics
}

func (t *Manager) HandlePublishRec(topicName string, pubrec *packets.Pubrec) error {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.QoS2); ok {
			t.HandlePublishRec(pubrec)
			return nil
		}
		logger.Logger.Warn("handle publish Rec failed, handle type error not QoS2")
		return errs.ErrTopicQoSNotSupport
	}

	logger.Logger.Warn("handle publish Rec failed, topic not exists")

	return errs.ErrTopicNotExistsInSubTopics
}

func (t *Manager) HandelPublishComp(topicName string, pubcomp *packets.Pubcomp) error {
	if topicValue, ok := t.topic[topicName]; ok {
		if t, ok := topicValue.(topic.QoS2); ok {
			t.HandlePublishComp(pubcomp)
			return nil
		}
		logger.Logger.Warn("handle publish Comp failed, handle type error not QoS2")
		return errs.ErrTopicQoSNotSupport

	}
	logger.Logger.Warn("handle publish Comp failed, topic not exists ")
	return errs.ErrTopicNotExistsInSubTopics
}

func (t *Manager) AddTopic(topicName string, topic topic.Topic) error {
	if existTopic, ok := t.topic[topicName]; ok {
		if err := existTopic.Close(); err != nil {
			logger.Logger.Error("topic manager close close failed", zap.Error(err), zap.String("topic", topicName), zap.String("client uid", pkg.GetClientUID(t.ctx)))
			return err
		}
	}
	t.topic[topicName] = topic
	go func() {
		if err := topic.Start(t.ctx); err != nil {
			logger.Logger.Error("start topic failed", zap.Error(err), zap.String("topic", topicName), zap.String("client uid", pkg.GetClientUID(t.ctx)))
		}
	}()
	return nil
}

func (t *Manager) DeleteTopic(topicName string) {
	if tt, ok := t.topic[topicName]; ok {
		if err := tt.Close(); err != nil {
			logger.Logger.Error("client topic manager close topic failed", zap.Error(err),
				zap.String("topic", topicName),
				zap.String("client uid", pkg.GetClientUID(t.ctx)))
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
	for topicName, topicInstance := range t.topic {
		unfinishedMessage[topicName] = topicInstance.GetUnFinishedMessage()
	}
	return unfinishedMessage
}

func (t *Manager) Publish(topic string, message *packet.Message) error {
	if _, ok := t.topic[topic]; !ok {
		return errs.ErrTopicNotExistsInSubTopics
	}
	return t.topic[topic].Publish(message)
}

func (t *Manager) Meta() []topic.Meta {
	var meta = make([]topic.Meta, 0)
	for _, to := range t.topic {
		meta = append(meta, to.Meta())
	}
	return meta
}

func CreateTopic(client client.PacketWriter, meta *topic.Meta) topic.Topic {
	var (
		topicName = meta.Topic
		qos       = byte(meta.QoS)
	)
	switch qos {
	case broker.QoS0:
		return NewQoS0(meta, client, message_source.NewEventSource(topicName, event.GlobalEvent))
	case broker.QoS1:
		return NewQoS1(meta, client, message_source.NewStoreSource(topicName, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	case broker.QoS2:
		return NewQoS2(meta, client, message_source.NewStoreSource(topicName, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	default:
		logger.Logger.Error("topic manager create topic failed, qos not support", zap.String("topic", topicName), zap.Uint8("qos", qos), zap.String("client id", client.GetID()))
		return nil
	}
}

func CreateTopicFromSession(client client.PacketWriter, meta *topic.Meta, unfinished []*packet.Message) topic.Topic {
	var (
		topicName       = meta.Topic
		qos             = byte(meta.QoS)
		latestMessageID = meta.LatestMessageID
	)
	switch byte(meta.QoS) {
	case broker.QoS0:
		return NewQoS0(meta, client, message_source.NewEventSource(topicName, event.GlobalEvent))
	case broker.QoS1:
		return NewQoS1(meta, client, message_source.NewStoreSource(topicName, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, QoS1WithLatestMessageID(latestMessageID))
	case broker.QoS2:
		return NewQoS2(meta, client, message_source.NewStoreSource(topicName, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, QoS2WithLatestMessageID(latestMessageID))
	default:
		logger.Logger.Error("create topicName failed, qos not support", zap.String("topicName", topicName), zap.Uint8("qos", qos))
		return nil
	}
}
