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
		messageID   string
	)
	publishComp.PacketID = publishRel.PacketID
	publishPacket, ok := client.QoS2.HandlePubRel(publishRel)
	if !ok {
		publishRel.ReasonCode = packets.PubackUnspecifiedError
		client.WritePacket(publishComp)
		return
	}

	subTopics := broker.subTree.MatchTopic(publishPacket.Topic)
	messageID, err = broker.store.StorePublishPacket(subTopics, &packet2.PublishMessage{
		ClientID:      client.GetID(),
		PublishPacket: publishPacket,
	})
	if err != nil {
		logger.Logger.Error("store publish packet error", zap.Error(err))
		publishRel.ReasonCode = packets.PubackUnspecifiedError
	}
	if !publishPacket.Retain {
		client.WritePacket(publishComp)
		return
	}
	// if retain is true and payload is not empty, then create retain message id
	if len(publishPacket.Payload) > 0 {
		for topic := range subTopics {
			if err = broker.state.CreateRetainMessageID(topic, messageID); err != nil {
				logger.Logger.Error("create retain message id error", zap.Error(err), zap.String("topic", topic))
				publishRel.ReasonCode = packets.PubackUnspecifiedError
			}
		}
	} else {
		// if retain is true and payload is empty, then delete retain message id
		if err = broker.state.DeleteRetainMessageID(publishPacket.Topic); err != nil {
			logger.Logger.Error("delete retain message id error", zap.Error(err))
			publishRel.ReasonCode = packets.PubackUnspecifiedError
		}
	}
	client.WritePacket(publishComp)
	return
}
