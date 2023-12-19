package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type UnsubHandler struct {
}

func NewUnsubHandler() *UnsubHandler {
	return &UnsubHandler{}
}
func (u *UnsubHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	// TODO: there should check packetID is valid, no other packet use it.
	var (
		packet   = rawPacket.Content.(*packets.Unsubscribe)
		packetID = packet.PacketID
		unsubAck = packets.NewControlPacket(packets.UNSUBACK).Content.(*packets.Unsuback)
	)

	// update broker's subTree.
	if err = broker.subTree.DeleteSub(client.ID, packet.Topics); err != nil {
		logger.Logger.Error("delete sub error", zap.Error(err), zap.String("client", client.MetaString()), zap.Strings("topics", packet.Topics))
		for range packet.Topics {
			unsubAck.Reasons = append(unsubAck.Reasons, packets.UnsubackUnspecifiedError)
		}
		unsubAck.PacketID = packetID
		broker.writePacket(client, unsubAck)
	}
	return err
}
