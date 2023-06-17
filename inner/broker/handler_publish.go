package broker

import (
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
		packet      = rawPacket.Content.(*packets.Publish)
		topic       = packet.Topic
		qos         = uint8(packet.QoS)
		pubAck      = packets.NewControlPacket(packets.PUBACK).Content.(*packets.Puback)
		subClients  = broker.subTree.Match(topic)
		encodedData []byte
		err         error
		messageID   string
	)
	pubAck.PacketID = packet.PacketID
	if len(subClients) == 0 {
		pubAck.ReasonCode = packets.PubackNoMatchingSubscribers
		broker.writePacket(client, pubAck)
		return
	}
	event.EmitTopicPublishEvent(topic, packet)
	if qos == pkg.QoS0 {
		// FIXME: cluster event
		return
	}
	if broker.store == nil {
		logger.Logger.Error("store is nil")
		// TODO: send message to sub topic client
		return
	}

	if qos == pkg.QoS1 && len(subClients) != 0 {
		if encodedData, err = pkg.Encode(packet); err == nil {
			messageID, err = broker.store.CreatePacket(topic, encodedData)
			if err != nil {
				logger.Logger.Error("create packet to store error = ", err.Error())
			} else {
				logger.Logger.Debug("create packet to store id = ", messageID, " topic = ", topic)
				pubAck.ReasonCode = packets.PubackSuccess
				emitPublishToClients(topic, messageID, subClients)
			}
		}
	}
	broker.writePacket(client, pubAck)
}

func emitPublishToClients(topic, messageID string, clients map[string]int32) {
	for clientID := range clients {
		event.EmitPublishToClientEvent(clientID, topic, messageID)
	}
}
