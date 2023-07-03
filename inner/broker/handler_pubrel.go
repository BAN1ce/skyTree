package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
)

type PublishRel struct {
}

func NewPublishRel() *PublishRel {
	return &PublishRel{}
}

func (p *PublishRel) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		publishRel = rawPacket.Content.(*packets.Pubrel)
	)
	if publishPacket, ok := client.QoS2.HandlePubRel(publishRel); ok {
		_, err := storePublishPacket(broker, client, publishPacket)
		if err != nil {
			publishRel.ReasonCode = packets.PubackUnspecifiedError
		}
		publishComp := packet2.NewPublishComp()
		publishComp.PacketID = publishRel.PacketID
		client.WritePacket(publishComp)
	}

}
