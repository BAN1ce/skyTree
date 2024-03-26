package topic

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/client"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

type QoS1Option func(q *QoS1)

func QoS1WithLatestMessageID(messageID string) QoS1Option {
	return func(q *QoS1) {
		q.meta.LatestMessageID = messageID
	}
}

type QoS1 struct {
	ctx               context.Context
	cancel            context.CancelFunc
	meta              *topic.Meta
	publishChan       chan *packet.Message
	client            *WithRetryClient
	messageSource     broker.MessageSource
	unfinishedMessage []*packet.Message
}

func (q *QoS1) GetUnfinishedMessage() []*session.UnFinishedMessage {
	//TODO implement me
	panic("implement me")
}

func NewQoS1(meta *topic.Meta, writer client.PacketWriter, messageSource broker.MessageSource, unfinishedMessage []*packet.Message, options ...QoS1Option) *QoS1 {
	q := &QoS1{
		meta:          meta,
		client:        NewQoSWithRetry(NewClient(writer, meta), nil),
		messageSource: messageSource,
	}
	q.unfinishedMessage = unfinishedMessage
	for _, op := range options {
		op(q)
	}
	return q
}

func (q *QoS1) Start(ctx context.Context) error {
	q.ctx, q.cancel = context.WithCancel(ctx)
	if q.meta.WindowSize == 0 {
		// FIXME: config.GetTopic().WindowSize,use client or another config
		q.meta.WindowSize = config.GetTopic().WindowSize
	}
	q.publishChan = make(chan *packet.Message, max(len(q.unfinishedMessage), q.meta.WindowSize))
	//q.publishChan = make(chan *packet.Message, 0)

	for _, msg := range FillUnfinishedMessage(q.ctx, q.unfinishedMessage, q.messageSource) {
		q.publishChan <- msg
	}
	clear(q.unfinishedMessage)
	q.listenPublishChan()
	return nil
}

func (q *QoS1) HandlePublishAck(publishAck *packets.Puback) {
	q.client.HandlePublishAck(publishAck)
}

func (q *QoS1) listenPublishChan() {
	var (
		delayTime = 5 * time.Second
	)
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			msg.SetSubIdentifier(byte(q.meta.Identifier))
			if err := q.client.Publish(msg); err != nil {
				logger.Logger.Warn("QoS1: publish error = ", zap.Error(err))
			}
			if !msg.IsFromSession() {
				q.meta.LatestMessageID = msg.MessageID
			}
		default:
			message, _, err := q.messageSource.NextMessages(q.ctx, q.meta.WindowSize, q.meta.LatestMessageID, false)
			if err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("store", q.meta.Topic))
				time.Sleep(delayTime)
				delayTime *= 2
				if delayTime > 5*time.Minute {
					delayTime = 5 * time.Second
				}
				continue
			}
			delayTime = 5 * time.Second
			for _, m := range message {
				q.writeToPublishChan(m)
			}
		}
	}
}

func (q *QoS1) writeToPublishChan(message *packet.Message) {
	if err := q.Publish(message); err != nil {
		logger.Logger.Warn("QoS1: write to publishChan error = ", zap.Error(err), zap.String("store", q.meta.Topic))
	}
}

func (q *QoS1) Close() error {
	if q.ctx == nil || q.cancel == nil {
		return fmt.Errorf("QoS1: ctx is nil")
	}
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	q.cancel()
	return nil
}

func (q *QoS1) Publish(publish *packet.Message) error {
	if q.ctx.Err() != nil {
		return q.ctx.Err()
	}
	select {
	case <-q.ctx.Done():
		return q.ctx.Err()
	case q.publishChan <- publish:
		return nil
	default:
		logger.Logger.Warn("QoS1: publishChan is full", zap.String("store", q.meta.Topic))
		return fmt.Errorf("QoS1: publishChan is full")
	}
}

func (q *QoS1) GetUnFinishedMessage() []*packet.Message {
	return q.client.GetUnFinishedMessage()
}

func (q *QoS1) Meta() topic.Meta {
	return *q.meta
}
