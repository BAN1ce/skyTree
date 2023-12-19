package topic

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

// QoS0 is topic with QoS0
type QoS0 struct {
	ctx           context.Context
	cancel        context.CancelFunc
	topic         string
	subOption     *packets.SubOptions
	messageSource broker.MessageSource
	client        broker.Client
}

func (q *QoS0) GetUnfinishedMessage() []*session.UnFinishedMessage {
	return nil
}

func NewQoS0(subOption *packets.SubOptions, writer broker.PacketWriter, messageSource broker.MessageSource) *QoS0 {
	q := &QoS0{
		topic:         subOption.Topic,
		subOption:     subOption,
		messageSource: messageSource,
		client:        NewClient(writer, subOption),
	}
	return q
}

// Start starts the QoS0 topic, and it will block until the context is done.
// It will create a publish event listener to listen the publish event of the store.
func (q *QoS0) Start(ctx context.Context) error {
	q.ctx, q.cancel = context.WithCancel(ctx)
	messageChan, err := q.messageSource.ListenMessage(q.ctx)
	if err != nil {
		logger.Logger.Error("QoS0: listen message error", zap.Error(err))
		return err
	}
	logger.Logger.Debug("QoS0: start success", zap.String("topic", q.topic))
	defer func() {
		logger.Logger.Info("QoS0: close", zap.String("topic", q.topic),
			zap.String("clientID", q.GetID()))
	}()
	for {
		select {
		case <-q.ctx.Done():
			return nil
		case p, ok := <-messageChan:
			if !ok {
				return nil
			}
			_ = q.Publish(p)
		}
	}

}

// Close closes the QoS0
func (q *QoS0) Close() error {
	if q.ctx == nil {
		return fmt.Errorf("QoS0: ctx is nil, not start")
	}
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.cancel()
	return nil
}

func (q *QoS0) Publish(publish *packet.Message) error {
	return q.client.Publish(publish)
}

func (q *QoS0) GetID() string {
	return q.client.GetPacketWriter().GetID()
}

func (q *QoS0) GetUnFinishedMessage() []*packet.Message {
	return q.client.GetUnFinishedMessage()
}
