package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

type PublishRec struct {
}

func NewPublishRec() *PublishRec {
	return &PublishRec{}
}

func (p *PublishRec) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet, ok = rawPacket.Content.(*packets.Pubrec)
	)
	if !ok {
		logger.Logger.Error("convert to pubrec error")
		return
	}
	if packet.ReasonCode == packets.PubrecSuccess {
		client.HandlePubRec(packet)
	}
}
