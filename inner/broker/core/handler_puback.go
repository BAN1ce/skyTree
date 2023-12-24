package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type PubAck struct {
}

func NewPublishAck() *PubAck {
	return &PubAck{}
}

func (a *PubAck) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	var (
		packet = rawPacket.Content.(*packets.Puback)
	)
	if packet.ReasonCode == packets.PubackSuccess {
	} else {
		logger.Logger.Info("publish ack reason not success", zap.String("client", client.MetaString()),
			zap.Uint8("reason", packet.ReasonCode), zap.Uint16("packetID", packet.PacketID))
	}
	return err
}
