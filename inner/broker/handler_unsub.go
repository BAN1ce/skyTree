package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type UnsubHandler struct {
}

func NewUnsubHandler() *UnsubHandler {
	return &UnsubHandler{}
}
func (u *UnsubHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet   = rawPacket.Content.(*packets.Unsubscribe)
		packetID = packet.PacketID
		unsubAck = packets.NewControlPacket(packets.UNSUBACK).Content.(*packets.Unsuback)
	)
	// TODO: check unsub result
	broker.subTree.DeleteSub(client.ID, packet.Topics)
	for _, topic := range packet.Topics {
		client.HandleUnSub(topic)
		unsubAck.Reasons = append(unsubAck.Reasons, 0x00)
	}
	unsubAck.PacketID = packetID
	broker.writePacket(client, unsubAck)
}
