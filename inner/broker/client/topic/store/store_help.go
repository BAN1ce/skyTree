package store

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"go.uber.org/zap"
)

type HandleStoreReadDone func(latestMessageID string)

// Help is the help of the store.
// Read message from the store and write to the channel.
// It will listen the store event when store is empty, and read the message from the store when the event is triggered.
type Help struct {
	Event
	broker.ClientMessageStore
	handleStoreReadDone []HandleStoreReadDone
}

func NewStoreHelp(store broker.ClientMessageStore, event Event, done ...HandleStoreReadDone) *Help {
	return &Help{
		Event:               event,
		ClientMessageStore:  store,
		handleStoreReadDone: done,
	}
}

func (s *Help) ReadStore(ctx context.Context, topic, startMessageID string, size int, include bool, writer func(message *packet.PublishMessage)) (err error) {
	// FIXME: If consecutive errors, consider downgrading options
	var (
		total int
	)
	if startMessageID != "" {
		if err = s.readStoreWriteChan(ctx, topic, startMessageID, size, include, writer); err != nil {
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
			logger.Logger.Error("readStoreWriteChan error", zap.Any("i", i))
			return
		}
		id, _ := i[1].(string)
		if startMessageID == "" {
			startMessageID = id
		}
		if err = s.readStoreWriteChan(ctx1, topic, startMessageID, size, true, writer); err != nil {
			logger.Logger.Error("readStoreWriteChan error", zap.Error(err))
		}
		cancel()
	}
	s.CreateListenMessageStoreEvent(topic, f)
	<-ctx1.Done()
	s.DeleteListenMessageStoreEvent(topic, f)
	return
}

func (s *Help) readStoreWriteChan(ctx context.Context, topic string, id string, size int, include bool, writer func(message *packet.PublishMessage)) error {
	logger.Logger.Debug("readStoreWriteChan", zap.String("store", topic), zap.String("id", id), zap.Int("size", size), zap.Bool("include", include))
	var (
		message, err = s.ReadTopicMessagesByID(ctx, topic, id, size, include)
		messageID    string
	)
	if err != nil {
		return err
	}
	for _, m := range message {
		writer(&m)
	}
	for _, done := range s.handleStoreReadDone {
		done(messageID)
	}
	return nil

}
