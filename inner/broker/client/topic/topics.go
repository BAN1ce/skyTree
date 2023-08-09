package topic

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type Topic interface {
	Start(ctx context.Context)
	Close() error
	HandlePublishAck(puback *packets.Puback)
	HandlePublishRec(pubrec *packets.Pubrec)
	HandelPublishComp(pubcomp *packets.Pubcomp)
}

type PublishWriter interface {
	// WritePacket writes the packet to the writer.
	// Warning: packetID is original packetID, method should change it to the new one that does not used.
	WritePacket(packet packets.Packet)

	GetID() string
	Close() error
}

type Option func(topics *Topics)

func WithStore(store pkg.ClientMessageStore) Option {
	return func(topic *Topics) {
		topic.store = store
	}
}

func WithWriter(writer PublishWriter) Option {
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
	session    pkg.SessionTopic
	store      pkg.ClientMessageStore
	writer     PublishWriter
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

func NewTopicWithSession(ctx context.Context, session pkg.SessionTopic, op ...Option) *Topics {
	t := NewTopics(ctx, op...)
	t.session = session
	for topic, qos := range session.ReadSubTopics() {
		logger.Logger.Debug("read store from client.proto = ", zap.String("store", topic), zap.Int32("qos", qos))
		t.CreateTopic(topic, pkg.Int32ToQoS(qos))

	}
	return t
}

func (t *Topics) CreateTopic(topicName string, qos pkg.QoS) {
	var (
		topic Topic
	)
	if t, ok := t.topic[topicName]; ok {
		if err := t.Close(); err != nil {
			logger.Logger.Warn("close store error = ", zap.Error(err))
		}
	}
	switch qos {
	case pkg.QoS0:
		logger.Logger.Debug("create store with QoS0", zap.String("store", topicName))
		topic = t.createQoS0Topic(topicName)
	case pkg.QoS1:
		logger.Logger.Debug("create store with QoS1", zap.String("store", topicName))
		topic = t.createQoS1Topic(topicName, t.writer)
	case pkg.QoS2:
		logger.Logger.Debug("create store with QoS2", zap.String("store", topicName))
		topic = t.createQoS2Topic(topicName, t.writer)
	default:
		logger.Logger.Warn("create store with wrong QoS ", zap.Uint8("qos", uint8(qos)))
		return
	}
	t.session.CreateSubTopic(topicName, int32(qos))
	t.topic[topicName] = topic
	go topic.Start(t.ctx)
}

func (t *Topics) HandlePublishAck(topic string, puback *packets.Puback) {
	if topic, ok := t.topic[topic]; ok {
		topic.HandlePublishAck(puback)
	}
}

func (t *Topics) HandlePublishRec(topic string, pubrec *packets.Pubrec) {
	if topic, ok := t.topic[topic]; ok {
		topic.HandlePublishRec(pubrec)
	}
}

func (t *Topics) HandelPublishComp(topic string, pubcomp *packets.Pubcomp) {
	if topic, ok := t.topic[topic]; ok {
		topic.HandelPublishComp(pubcomp)
	}
}

func (t *Topics) createQoS0Topic(topicName string) Topic {
	return NewQoS0(topicName, t.writer, event.GlobalEvent)
}

func (t *Topics) createQoS1Topic(topicName string, writer PublishWriter) Topic {
	return NewQos1(topicName, writer, NewStoreHelp(t.store, event.GlobalEvent, func(latestMessageID string) {
		t.session.SetTopicLatestPushedMessageID(topicName, latestMessageID)
	}), t.session)
}

func (t *Topics) createQoS2Topic(topicName string, writer PublishWriter) Topic {
	return NewQos2(topicName, writer, NewStoreHelp(t.store, event.GlobalEvent), t.session)
}

func (t *Topics) DeleteTopic(topicName string) {
	if _, ok := t.topic[topicName]; ok {
		if err := t.topic[topicName].Close(); err != nil {
			logger.Logger.Warn("topics close store failed", zap.Error(err), zap.String("store", topicName))
		}
		delete(t.topic, topicName)
	}
}

func (t *Topics) Close() error {
	// for no sub client
	if t == nil {
		return nil
	}
	for topicName, topic := range t.topic {
		t.DeleteTopic(topicName)
		switch tmp := topic.(type) {
		case *QoS1:
			t.session.CreateTopicUnAckMessageID(topicName, tmp.GetUnAckedMessageID())
		case *QoS2:
			for _, packet := range tmp.GetUnFinish() {
				if packet.IsReceived {
					t.session.CreateTopicUnCompPacketID(topicName, []string{packet.PacketID})
				} else {
					t.session.CreateTopicUnRecPacketID(topicName, []string{packet.MessageID})
				}
			}
		case *QoS0:
		default:
			logger.Logger.Error("unknown topic type", zap.String("type", fmt.Sprintf("%T", tmp)))
		}
	}
	return nil
}
