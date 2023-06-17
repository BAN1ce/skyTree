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
	)
	broker.subTree.CreateSub(client.ID, packet.Subscriptions)
	var (
		subAck = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = packet.PacketID
	result := client.HandleSub(packet)
	for topic := range packet.Subscriptions {
		subAck.Reasons = append(subAck.Reasons, result[topic])
	}
	broker.writePacket(client, subAck)
}
