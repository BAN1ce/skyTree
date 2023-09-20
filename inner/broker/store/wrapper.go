package store

import (
	"bytes"
	"context"
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/errs"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

// Wrapper is a wrapper of pkg.Store
type Wrapper struct {
}

func NewStoreWrapper() *Wrapper {
	return &Wrapper{}
}

// StorePublishPacket stores the published packet to store
// if topics is empty, return error
// topics include the origin topic and the topic of the wildcard subscription
// and emit store event
func (s *Wrapper) StorePublishPacket(topics map[string]int32, packet *packets.Publish) (messageID string, err error) {
	var (
		// there doesn't use bytes.BufferPool, because the store maybe async
		encodedData = bytes.NewBuffer(nil)
		topic       = packet.Topic
	)
	if len(topics) == 0 {
		return "", errs.ErrStoreTopicsEmpty
	}
	// publish packet encode to bytes
	if err := broker.Encode(DefaultSerializerVersion, &packet2.PublishMessage{
		PublishPacket: packet,
		TimeStamp:     time.Now().Unix(),
	}, encodedData); err != nil {
		return "", err
	}

	for topic = range topics {
		// store message bytes
		messageID, err = DefaultMessageStore.CreatePacket(topic, encodedData.Bytes())
		if err != nil {
			logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("store", topic))
		} else {
			logger.Logger.Debug("create packet to store success", zap.String("store", topic), zap.String("messageID", messageID))
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
func ReadPublishMessage(ctx context.Context, topic, startMessageID string, size int, include bool, writer func(message *packet2.PublishMessage)) (err error) {
	// FIXME: If consecutive errors, consider downgrading options
	var (
		total int
	)
	if startMessageID != "" {
		if err = readStoreWriteToWriter(ctx, topic, startMessageID, size, include, writer); err != nil {
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
		if err = readStoreWriteToWriter(ctx1, topic, startMessageID, size, true, writer); err != nil {
			logger.Logger.Error("readStoreWriteToWriter error", zap.Error(err))
		}
		cancel()
	}
	DefaultMessageStoreEvent.CreateListenMessageStoreEvent(topic, f)
	<-ctx1.Done()
	DefaultMessageStoreEvent.DeleteListenMessageStoreEvent(topic, f)
	return
}

// readStoreWriteToWriter read message from store and write to writer
func readStoreWriteToWriter(ctx context.Context, topic string, id string, size int, include bool, writer func(message *packet2.PublishMessage)) error {
	var (
		message, err = DefaultMessageStore.ReadTopicMessagesByID(ctx, topic, id, size, include)
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
			writer(&m)
		}
	}
	return nil
}

func ReadTopicWillMessage(ctx context.Context, topic, messageID string, writer func(message *packet2.PublishMessage)) error {
	var (
		// TODO: limit maybe not enough
		message, err = DefaultMessageStore.ReadTopicMessagesByID(ctx, topic, messageID, 1, true)
	)
	if err != nil {
		return err
	}
	logger.Logger.Debug("store help read publish message and write to channel",
		zap.String("store", topic),
		zap.Int("got message size", len(message)))
	for _, m := range message {
		if m.Will {
			writer(&m)
		}
	}
	return nil
}

func DeleteTopicMessageID(ctx context.Context, topic, messageID string) error {
	return DefaultMessageStore.DeleteTopicMessageID(ctx, topic, messageID)
}
