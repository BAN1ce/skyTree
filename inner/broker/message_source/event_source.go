package message_source

import (
	"context"
	"errors"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"go.uber.org/zap"
	"sync"
)

type EventSource struct {
	topic           string
	publishListener broker.PublishListener
	messageChan     chan *packet.Message
	onceClose       sync.Once
}

func NewEventSource(topic string, listener broker.PublishListener) broker.MessageSource {
	return &EventSource{
		topic:           topic,
		publishListener: listener,
		messageChan:     make(chan *packet.Message, 10),
	}
}

func (s *EventSource) Close() error {
	s.publishListener.DeletePublishEvent(s.topic, s.handler)
	return nil
}

func (s *EventSource) ListenMessage(context.Context) (<-chan *packet.Message, error) {
	if s == nil {
		return nil, errors.New("event source is nil")
	}
	s.publishListener.CreatePublishEvent(s.topic, s.handler)
	return s.messageChan, nil
}

func (s *EventSource) NextMessages(ctx context.Context, n int, startMessageID string, include bool) ([]*packet.Message, int, error) {
	var (
		msg = make([]*packet.Message, 0, n)
	)
	for i := 0; i < n; i++ {
		select {
		case m, ok := <-s.messageChan:
			if !ok {
				break
			}
			msg = append(msg, m)
		case <-ctx.Done():
			return msg, len(msg), nil
		default:
			return msg, len(msg), nil
		}
	}
	return msg, len(msg), nil

}

// handler is the handler of the topic, it will be called when the published packet event is triggered.
func (s *EventSource) handler(i ...interface{}) {
	defer func() {
		if err := recover(); err != nil {
			logger.Logger.Error("QoS0: handler panic", zap.Any("err", err))
		}
	}()
	if len(i) == 2 {
		p, ok := i[1].(*packet.Message)
		if !ok {
			logger.Logger.Error("ListenTopicPublishEvent: type error")
		}
		if p == nil || p.PublishPacket == nil {
			logger.Logger.Error("ListenTopicPublishEvent: publish packet is nil")
			return
		}
		// copy the published packet and set the QoS to QoS0
		var publishPacket = pool.PublishPool.Get()
		pool.CopyPublish(publishPacket, p.PublishPacket)
		publishPacket.QoS = broker.QoS0
		select {
		case s.messageChan <- p:
		default:
			logger.Logger.Warn("drop message")
		}
	}
}
