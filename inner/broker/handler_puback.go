package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

type PubAck struct {
}

func NewPublishAck() *PubAck {
	return &PubAck{}
}

func (a *PubAck) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet = rawPacket.Content.(*packets.Puback)
	)
	if packet.ReasonCode == packets.PubackSuccess {
		client.HandlePubAck(packet)
	} else {
		logger.Logger.Info("publish ack reason code = ", packet.ReasonCode, " packet id = ", packet.PacketID, " client id = ", client.ID)
	}
}
