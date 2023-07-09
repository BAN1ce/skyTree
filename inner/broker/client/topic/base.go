package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

func copyPublish(publish *packets.Publish) *packets.Publish {
	var publishPacket = pool.PublishPool.Get()
	pool.CopyPublish(publishPacket, publish)
	publishPacket.QoS = pkg.QoS0
	return publishPacket
}

type StoreHelp struct {
	lastMessageIDFromStore string
	StoreEvent
	pkg.ClientMessageStore
}

func NewStoreHelp(store pkg.ClientMessageStore, event StoreEvent, lastMessageID string) *StoreHelp {
	return &StoreHelp{
		lastMessageIDFromStore: lastMessageID,
		StoreEvent:             event,
		ClientMessageStore:     store,
	}
}

func (s *StoreHelp) readStore(ctx context.Context, ch chan<- *packet.PublishMessage, topic string, size int, include bool) error {
	if s.lastMessageIDFromStore != "" {
		if total, err := s.readStoreWriteChan(ctx, ch, topic, s.lastMessageIDFromStore, size, include); err != nil {
			return err
		} else if total != 0 {
			return err
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
		if s.lastMessageIDFromStore == "" {
			s.lastMessageIDFromStore = id
		}
		if _, err := s.readStoreWriteChan(ctx1, ch, topic, s.lastMessageIDFromStore, size, true); err != nil {
			logger.Logger.Error("readStoreWriteChan error", zap.Error(err))
		}
		cancel()
	}
	s.CreateListenMessageStoreEvent(topic, f)
	<-ctx1.Done()
	s.DeleteListenMessageStoreEvent(topic, f)
	return nil
}

func (s *StoreHelp) readStoreWriteChan(ctx context.Context, ch chan<- *packet.PublishMessage, topic string, id string, size int, include bool) (int, error) {
	logger.Logger.Debug("readStoreWriteChan", zap.String("topic", topic), zap.String("id", id), zap.Int("size", size), zap.Bool("include", include))
	var (
		message, err = s.ReadTopicMessagesByID(ctx, topic, id, size, include)
		i            int
	)
	if err != nil {
		return i, err
	}
	for _, m := range message {
		select {
		case ch <- &m:
			i++
			s.lastMessageIDFromStore = m.MessageID
		default:
			return i, nil
		}
	}
	return i, nil
}
