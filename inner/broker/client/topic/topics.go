package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/client/qos"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"io"
)

type Option func(topics *Topics)

func WithStore(store pkg.ClientMessageStore) Option {
	return func(topic *Topics) {
		topic.store = store
	}
}

func WithWriter(writer io.Writer) Option {
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
	writer     io.Writer
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

func NewTopicWithSession(ctx context.Context, session pkg.SessionTopic, op ...Option) (*Topics, error) {
	t := NewTopics(ctx, op...)
	t.session = session
	for topic, qos := range session.ReadSubTopics() {
		logger.Logger.Info("topic = ", topic, " qos = ", qos)
		if err := t.CreateTopic(topic, pkg.Int32ToQoS(qos)); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *Topics) CreateTopic(topicName string, qos pkg.QoS) error {
	var (
		topic Topic
	)
	if t, ok := t.topic[topicName]; ok {
		if err := t.Close(); err != nil {
			logger.Logger.Info("close topic error = ", err.Error())
		}
	}
	switch qos {
	case pkg.QoS0:
		topic = t.createQoS0Topic(topicName)
	case pkg.QoS1:
		panic("implement me")
	case pkg.QoS2:
		panic("implement me")
	default:
		return errs.ErrInvalidQoS
	}
	t.session.CreateSubTopics(topicName, int32(qos))
	t.topic[topicName] = topic
	topic.Start(t.ctx)
	return nil
}

func (t *Topics) createQoS0Topic(topicName string) Topic {
	return qos.NewQoS0(topicName, t.writer)
}

func (t *Topics) createQoS1Topic(topicName string, writer qos.Writer) Topic {
	return qos.NewQos1(topicName, 10, t.store, t.session, writer)
}

func (t *Topics) createQoS2Topic(topicName string) Topic {
	panic("implement me")
}

func (t *Topics) DeleteTopic(topicName string) {
	if _, ok := t.topic[topicName]; ok {
		if err := t.topic[topicName].Close(); err != nil {
			logger.Logger.Error("release topic error = ", err.Error())
		}
		delete(t.topic, topicName)
	}
}

func (t *Topics) Close() error {
	for _, topic := range t.topic {
		if err := topic.Close(); err != nil {
			logger.Logger.Error("close topic error = ", err.Error())
		}
	}
	return nil
}
