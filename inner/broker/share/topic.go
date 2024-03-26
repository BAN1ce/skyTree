package share

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	topic2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"go.uber.org/zap"
	"sync"
)

type Topic struct {
	closeChan chan string
	id        string
}

func (t *Topic) Start(ctx context.Context) error {
	return nil
}

func (t *Topic) Close() error {
	t.closeChan <- t.id
	return nil
}

func (t *Topic) Meta() topic2.Meta {
	// TODO: implement me
	return topic2.Meta{}
}

func (t *Topic) Publish(publish *packet.Message) error {
	return nil
}

func (t *Topic) GetUnFinishedMessage() []*packet.Message {
	return nil
}

// $share/{ShareName}/{filter}

type Dispatch struct {
	ctx             context.Context
	cancel          context.CancelFunc
	topic           string
	shareTopic      string
	shareName       string
	subTopic        string
	mux             sync.RWMutex
	readMux         sync.RWMutex
	ID              string
	messageSource   broker.MessageSource
	latestMessageID string
	queue           *Queue // *packet.Message
	client          map[string]topic2.Topic
}

func NewDispatch(topic, latestMessageID string, source broker.MessageSource) *Dispatch {
	return &Dispatch{
		topic:           topic,
		messageSource:   source,
		latestMessageID: latestMessageID,
		queue:           NewQueue(),
		client:          make(map[string]topic2.Topic),
	}
}

func (d *Dispatch) NewTopic(clientID string) topic2.Topic {
	return &Topic{
		closeChan: make(chan string),
		id:        clientID,
	}
}

func (d *Dispatch) Start(ctx context.Context) {
	d.ctx, d.cancel = context.WithCancel(ctx)
}

func (d *Dispatch) nextMessage(ctx context.Context, n int) ([]*packet.Message, error) {
	var (
		message []*packet.Message
	)
	d.readMux.RLock()
	if d.queue.Len() != 0 {
		defer d.readMux.RUnlock()
		return d.queue.PopQueue(n), nil
	}

	d.readMux.RUnlock()
	d.readMux.Lock()
	defer d.readMux.Unlock()
	if d.queue.Len() != 0 {
		return d.queue.PopQueue(n), nil
	}

	message, _, err := d.messageSource.NextMessages(ctx, 20, d.latestMessageID, false)
	if err != nil {
		return nil, err
	}
	if len(message) == 0 {
		return nil, nil
	}
	d.queue.AppendMessage(message)
	d.latestMessageID = message[len(message)-1].MessageID
	return d.queue.PopQueue(n), nil
}

func (d *Dispatch) Close() error {
	d.cancel()
	return nil
}

func (d *Dispatch) ShareTopicAddClient(client broker.ShareClient, meta *topic2.Meta) (topic2.Topic, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	if client, ok := d.client[client.GetID()]; ok {
		client.Close()
	}
	switch byte(meta.QoS) {
	case broker.QoS0:
		subClient := topic.NewQoS0(meta, client, d)
		d.client[client.GetID()] = subClient
		go subClient.Start(d.ctx)

	case broker.QoS1:
		subClient := topic.NewQoS1(meta, client, d, nil)
		d.client[client.GetID()] = subClient
		go subClient.Start(d.ctx)

	case broker.QoS2:
		subClient := topic.NewQoS2(meta, client, d, nil)
		d.client[client.GetID()] = subClient
		go subClient.Start(d.ctx)
	}
	return d.NewTopic(client.GetID()), nil
}

func (d *Dispatch) ShareTopicRemoveClient(id string) {
	d.mux.Lock()
	defer d.mux.Unlock()
	if client, ok := d.client[id]; ok {
		if err := client.Close(); err != nil {
			logger.Logger.Error("close client error", zap.Error(err))
		}
		var message []*packet.Message
		for _, m := range client.GetUnFinishedMessage() {
			message = append(message, m)
		}
		d.queue.PutBackMessage(message)
	}
}

func (d *Dispatch) Publish(publish *packet.Message) error {
	//TODO implement me
	panic("implement me")
}

func (d *Dispatch) NextMessages(ctx context.Context, n int, startMessageID string, include bool) ([]*packet.Message, int, error) {
	message, err := d.nextMessage(ctx, n)
	return message, len(message), err
}

func (d *Dispatch) ListenMessage(ctx context.Context) (<-chan *packet.Message, error) {
	var (
		size = 10
		c    = make(chan *packet.Message, size)
	)
	go func(ctx context.Context, c chan *packet.Message, size int) {
		defer close(c)
		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ctx.Done():
				return
			default:
				if m, err := d.nextMessage(d.ctx, size); err != nil {
					return
				} else {
					c <- m[0]
				}
			}
		}
	}(ctx, c, size)
	return c, nil
}
