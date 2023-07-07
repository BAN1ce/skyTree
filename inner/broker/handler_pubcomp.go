package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

type PublishComp struct {
}

func NewPublishComp() *PublishComp {
	return &PublishComp{}
}

func (a *PublishComp) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet, ok = rawPacket.Content.(*packets.Pubcomp)
	)
	if !ok {
		logger.Logger.Error("convert to pubcomp error")
		return
	}
	if packet.ReasonCode == packets.PubcompSuccess {
		client.HandlePubComp(packet)
	}
}
