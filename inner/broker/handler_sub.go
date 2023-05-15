package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type SubHandler struct {
}

func NewSubHandler() *SubHandler {
	return &SubHandler{}
}
func (s *SubHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet, _ = rawPacket.Content.(*packets.Subscribe)
		session   = client.GetSession()
	)
	broker.subTree.CreateSub(client.ID, packet.Subscriptions)
	var (
		subAck = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = packet.PacketID
	for topic, op := range packet.Subscriptions {
		session.CreateSubTopics(topic, int32(op.QoS))
		subAck.Reasons = append(subAck.Reasons, 0x00)
	}
	broker.writePacket(client, subAck)
}
