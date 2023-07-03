package proxy

import (
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type StoreProxy struct {
	store pkg.Store
}

func (s *StoreProxy) StorePublishPacket(packet *packets.Publish) (string, error) {
	var (
		encodedData []byte
		err         error
		messageID   string
		topic       = packet.Topic
	)
	if encodedData, err = pkg.Encode(packet); err == nil {
		messageID, err = s.store.CreatePacket(topic, encodedData)
		if err != nil {
			logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("topic", topic))
		} else {
			logger.Logger.Debug("create packet to store success", zap.String("topic", topic),
				zap.String("messageID", messageID))
			event2.GlobalEvent.EmitStoreMessage(topic, messageID)
		}
	}
	return messageID, err
}
