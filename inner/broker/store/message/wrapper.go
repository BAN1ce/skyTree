package message

import (
	"bytes"
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/message_source"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"go.uber.org/zap"
)

// Wrapper is a wrapper of pkg.MessageStore
type Wrapper struct {
	broker.TopicMessageStore
	broker.MessageStoreEvent
}

func NewStoreWrapper(store broker.TopicMessageStore, event broker.MessageStoreEvent) *Wrapper {
	return &Wrapper{
		TopicMessageStore: store,
		MessageStoreEvent: event,
	}
}

// StorePublishPacket stores the published packet to store
// if topics is empty, return error
// topics include the origin topic and the topic of the wildcard subscription
// and emit store event
func (w *Wrapper) StorePublishPacket(topics map[string]int32, packet *packet2.Message) (messageID string, err error) {
	var (
		// there doesn't use bytes.BufferPool, because the store maybe async
		encodedData = bytes.NewBuffer(nil)
	)
	if len(topics) == 0 {
		logger.Logger.Info("store publish packet with empty topics")
		return "", nil
	}
	// publish packet encode to bytes
	if err := broker.Encode(store.DefaultSerializerVersion, packet, encodedData); err != nil {
		return "", err
	}

	for topic := range topics {
		// store message bytes
		messageID, err = w.CreatePacket(topic, encodedData.Bytes())
		if err != nil {
			logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("topic", topic))
		} else {
			logger.Logger.Debug("create packet to store success", zap.String("topic", topic), zap.String("messageID", messageID))
			// emit store event
			event2.GlobalEvent.EmitStoreMessage(topic, messageID)
		}
	}
	return messageID, err
}

// ReadPublishMessage reads the published message with the given topic and messageID from store
// if startMessageID is empty, read from the latest message
// if include is true, read from the startMessageID, otherwise read from the next message of startMessageID
// if include is true and startMessageID was the latest message, waiting for the next message by listening store event
// read message from store and write to writer
func (w *Wrapper) ReadPublishMessage(ctx context.Context, topic, startMessageID string, size int, include bool, writer func(message *packet2.Message)) (err error) {
	// FIXME: If consecutive errors, consider downgrading options
	var (
		total int
	)
	if startMessageID != "" {
		if err = w.readStoreWriteToWriter(ctx, topic, startMessageID, size, include, writer); err != nil {
			return
		} else if total != 0 {
			return
		}
	}
	var (
		f            func(...interface{})
		ctx1, cancel = context.WithCancel(ctx)
	)
	f = func(i ...interface{}) {
		if len(i) != 2 {
			logger.Logger.Error("readStoreWriteToWriter error", zap.Any("i", i))
			return
		}
		id, _ := i[1].(string)
		if startMessageID == "" {
			startMessageID = id
		}
		if err = w.readStoreWriteToWriter(ctx1, topic, startMessageID, size, true, writer); err != nil {
			logger.Logger.Error("readStoreWriteToWriter error", zap.Error(err))
		}
		cancel()
	}
	if ctx1.Err() != nil {
		return
	}
	w.CreateListenMessageStoreEvent(topic, f)
	<-ctx1.Done()
	w.DeleteListenMessageStoreEvent(topic, f)
	return
}

// readStoreWriteToWriter read message from store and write to writer
func (w *Wrapper) readStoreWriteToWriter(ctx context.Context, topic string, id string, size int, include bool, writer func(message *packet2.Message)) error {
	var (
		message, err = w.ReadTopicMessagesByID(ctx, topic, id, size, include)
	)
	if err != nil {
		return err
	}
	logger.Logger.Debug("store help read publish message and write to channel",
		zap.String("store", topic),
		zap.String("id", id),
		zap.Int("size", size),
		zap.Bool("include", include),
		zap.Int("got message size", len(message)))
	for _, m := range message {
		if !m.Will {
			writer(m)
		}
	}
	return nil
}

func (w *Wrapper) ReadTopicWillMessage(ctx context.Context, topic, messageID string, writer func(message *packet2.Message)) error {
	var (
		// TODO: limit maybe not enough
		message, err = w.ReadTopicMessagesByID(ctx, topic, messageID, 1, true)
	)
	if err != nil {
		return err
	}
	logger.Logger.Debug("store help read publish message and write to channel",
		zap.String("store", topic),
		zap.Int("got message size", len(message)))
	for _, m := range message {
		if m.Will {
			writer(m)
		}
	}
	return nil
}

func (w *Wrapper) DeleteTopicMessageID(ctx context.Context, topic, messageID string) error {
	return w.DeleteTopicMessageID(ctx, topic, messageID)
}

func (w *Wrapper) MakeMessageSource(topic string) broker.MessageSource {
	return message_source.NewStoreSource(topic, w.TopicMessageStore, w.MessageStoreEvent)
}
