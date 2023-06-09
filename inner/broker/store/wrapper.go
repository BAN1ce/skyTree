package store

import (
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

// Wrapper is a wrapper of pkg.Store
type Wrapper struct {
	pkg.Store
}

func NewStoreWrapper(store pkg.Store) *Wrapper {
	return &Wrapper{Store: store}
}

// StorePublishPacket stores the published packet to store
// and emit store event
func (s *Wrapper) StorePublishPacket(packet *packets.Publish) (messageID string, err error) {
	var (
		encodedData = pool.ByteBufferPool.Get()
		topic       = packet.Topic
	)
	defer pool.ByteBufferPool.Put(encodedData)
	_, err = packet.WriteTo(encodedData)
	if err != nil {
		logger.Logger.Error("encode packet error = ", zap.Error(err), zap.String("topic", topic))
		return
	}
	messageID, err = s.CreatePacket(topic, encodedData.Bytes())
	if err != nil {
		logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("topic", topic))
	} else {
		logger.Logger.Debug("create packet to store success", zap.String("topic", topic), zap.String("messageID", messageID))
		// emit store event
		event2.GlobalEvent.EmitStoreMessage(topic, messageID)
	}
	return messageID, err
}
