package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type PublishRel struct {
}

func NewPublishRel() *PublishRel {
	return &PublishRel{}
}

func (p *PublishRel) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		publishRel  = rawPacket.Content.(*packets.Pubrel)
		publishComp = packet2.NewPublishComp()
		err         error
	)
	publishComp.PacketID = publishRel.PacketID
	if publishPacket, ok := client.QoS2.HandlePubRel(publishRel); !ok {
		publishRel.ReasonCode = packets.PubackUnspecifiedError
	} else {
		_, err = broker.store.StorePublishPacket(publishPacket)
	}
	if err != nil {
		logger.Logger.Error("store publish packet error", zap.Error(err))
	}
	client.WritePacket(publishComp)

}
