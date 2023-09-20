package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type SubHandler struct {
}

func NewSubHandler() *SubHandler {
	return &SubHandler{}
}

func (s *SubHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet, _ = rawPacket.Content.(*packets.Subscribe)
		subAck    = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = packet.PacketID
	if err := broker.subTree.CreateSub(client.ID, packet.Subscriptions); err != nil {
		logger.Logger.Error("sub tree create sub failed", zap.Error(err))
		for range packet.Subscriptions {
			subAck.Reasons = append(subAck.Reasons, 0x80)
		}
		broker.writePacket(client, subAck)
		return
	}

	// client handle sub and create qos0,qos1,qos2 topic
	result := client.HandleSub(packet)
	for _, topic := range packet.Subscriptions {
		subAck.Reasons = append(subAck.Reasons, result[topic.Topic])
		// sub success, publish retain will message
		if result[topic.Topic] == 0x00 {
			for _, v := range broker.ReadTopicRetainWillMessage(topic.Topic) {
				if err := client.Publish(topic.Topic, v); err != nil {
					logger.Logger.Warn("publish retain will message failed", zap.String("topic", topic.Topic), zap.Error(err), zap.String("messageID", v.MessageID))
				}
			}
		}

	}
	broker.writePacket(client, subAck)
}
