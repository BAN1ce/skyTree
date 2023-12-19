package share

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/eclipse/paho.golang/packets"
	"sync"
)

type MessageSourceFactory interface {
	MakeMessageSource(topic string) broker.MessageSource
}
type ClientRemoveRequest interface {
	GetID() string
	GetUUID() string
	GetTopic() []string
}

type operation int

const (
	RemoveClient operation = iota
	AddClient
)

type clientOperation struct {
	topicName string
	subOption *packets.SubOptions
	operation operation
	clientID  string
	client    broker.ShareClient
}
type Manager struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	mux                  sync.RWMutex
	topic                map[string]*Dispatch
	messageSourceFactory MessageSourceFactory
	clientEvent          chan *clientOperation
}

func NewManager(factory MessageSourceFactory) *Manager {
	ctx, cancel := context.WithCancel(context.TODO())
	return &Manager{
		ctx:                  ctx,
		cancel:               cancel,
		topic:                make(map[string]*Dispatch),
		messageSourceFactory: factory,
	}
}

func (m *Manager) Sub(options packets.SubOptions, client broker.ShareClient) (topic.Topic, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	var (
		topicName = options.Topic
	)
	topic, ok := m.topic[topicName]
	if !ok {
		topic = NewDispatch(topicName, "", m.messageSourceFactory.MakeMessageSource(topicName))
		topic.Start(m.ctx)
		m.topic[topicName] = topic
	}
	t := topic.ShareTopicAddClient(client, options.QoS)
	return t, nil
}

func (m *Manager) UnSub(topic, clientID string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	t, ok := m.topic[topic]
	if !ok {
		return
	}
	t.ShareTopicRemoveClient(clientID)
}

func (m *Manager) CloseClient(i ...interface{}) {
	if len(i) != 1 {
		logger.Logger.Error("close client error, params length not equal 1")
		return
	}
	req, ok := i[0].(ClientRemoveRequest)
	if !ok {
		logger.Logger.Error("close client error, params type not equal ClientRemoveRequest")
		return
	}
	var (
		topic    = req.GetTopic()
		clientID = req.GetID()
	)
	m.mux.Lock()
	defer m.mux.Unlock()
	for _, t := range topic {
		topic, ok := m.topic[t]
		if !ok {
			continue
		}
		topic.ShareTopicRemoveClient(clientID)
	}
}

func (m *Manager) Close() error {
	m.cancel()
	return nil
}
