package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type SubHandler struct {
}

func NewSubHandler() *SubHandler {
	return &SubHandler{}
}

func (s *SubHandler) Handle(b *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	var (
		packet, _ = rawPacket.Content.(*packets.Subscribe)
		subAck    = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = packet.PacketID

	if err = b.subTree.CreateSub(client.ID, packet.Subscriptions); err != nil {
		logger.Logger.Error("sub tree create sub failed", zap.Error(err))
		for range packet.Subscriptions {
			subAck.Reasons = append(subAck.Reasons, 0x80)
		}
		b.writePacket(client, subAck)
		return err
	}

	shareSubscribePacket, _ := broker2.SplitShareAndNoShare(packet)

	// handle share sub, if share sub failed, return error.
	if err = s.handleShareSub(b, client, shareSubscribePacket); err != nil {
		return
	}
	return
}

func (s *SubHandler) handleShareSub(broker *Broker, client *client.Client, subscribe *packets.Subscribe) (err error) {
	for _, subOptions := range subscribe.Subscriptions {
		if _, err = broker.shareManager.Sub(subOptions, client); err != nil {
			logger.Logger.Error("share manager sub failed", zap.Error(err))
			return
		}
	}
	return
}
