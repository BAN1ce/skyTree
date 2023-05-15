package broker

import (
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/eclipse/paho.golang/packets"
)

type PublishHandler struct {
}

func NewPublishHandler() *PublishHandler {
	return &PublishHandler{}
}

func (p *PublishHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet     = rawPacket.Content.(*packets.Publish)
		topic      = packet.Topic
		qos        = uint8(packet.QoS)
		pubAck     = packets.NewControlPacket(packets.PUBACK).Content.(*packets.Puback)
		subClients = broker.subTree.Match(topic)
	)
	pubAck.PacketID = packet.PacketID
	if qos > config.GetPubMaxQos() {
		broker.disconnectClient(packets.DisconnectQoSNotSupported, client)
		return
	}
	if qos == pkg.QoS0 {
		// TODO: send message to sub topic client
	}
	if broker.store == nil {
		logger.Logger.Error("store is nil")
		// TODO: send message to sub topic client
		return
	}
	if len(subClients) == 0 {
		pubAck.ReasonCode = packets.PubackNoMatchingSubscribers
	}

	if qos == pkg.QoS1 && len(subClients) != 0 {
		// TODO: Message Expiry Interval
		id, err := broker.store.CreatePacket(topic, packet.Payload)
		if err != nil {
			logger.Logger.Error("create packet to store error = ", err.Error())
		} else {
			pubAck.ReasonCode = packets.PubackSuccess
		}
		for clientID := range subClients {
			event.EmitPublishToClientEvent(clientID, topic, id)
		}
		logger.Logger.Debug("create packet to store id = ", id, " topic = ", topic)
	}
	broker.writePacket(client, pubAck)
}
