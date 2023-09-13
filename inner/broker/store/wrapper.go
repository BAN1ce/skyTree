package store

import (
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

// Wrapper is a wrapper of pkg.Store
type Wrapper struct {
	broker.Store
}

func NewStoreWrapper(store broker.Store) *Wrapper {
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

	// publish packet encode to bytes
	if err := broker.Encode(broker.SerialVersion1, &packet2.PublishMessage{
		PublishPacket: packet,
		TimeStamp:     time.Now().Unix(),
	}, encodedData); err != nil {
		return "", err
	}
	// store message bytes
	messageID, err = s.CreatePacket(topic, encodedData.Bytes())
	if err != nil {
		logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("store", topic))
	} else {
		logger.Logger.Debug("create packet to store success", zap.String("store", topic), zap.String("messageID", messageID))
		// emit store event
		event2.GlobalEvent.EmitStoreMessage(topic, messageID)
	}
	return messageID, err
}
