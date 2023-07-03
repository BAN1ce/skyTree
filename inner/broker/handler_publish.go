package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/event"
	event2 "github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
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
		pubAck     = packet2.NewPublishAck()
		subClients = broker.subTree.Match(topic)
		err        error
	)
	// TODO: should emit all wildcard topic
	event.Event.Emit(event.ClientPublish, topic)
	event.Event.Emit(event.ReceivedTopicPublishEventName(topic), topic, packet)
	pubAck.PacketID = packet.PacketID

	switch qos {
	case pkg.QoS0:
		return
	case pkg.QoS1:
		if len(subClients) == 0 {
			pubAck.ReasonCode = packets.PubackNoMatchingSubscribers
			broker.writePacket(client, pubAck)
			return
		}
		if _, err = storePublishPacket(broker, client, packet); err != nil {
			pubAck.ReasonCode = packets.PubackUnspecifiedError
		} else {

			pubAck.ReasonCode = packets.PubackSuccess
		}
		client.WritePacket(pubAck)
	case pkg.QoS2:
		pubrec := packet2.NewPublishRec()
		if len(subClients) == 0 {
			pubrec.ReasonCode = packets.PubrecNoMatchingSubscribers
			broker.writePacket(client, pubrec)
			return
		}
		if client.QoS2.HandlePublish(packet) {
			pubrec.PacketID = packet.PacketID
			broker.writePacket(client, pubrec)
			return
		} else {
			logger.Logger.Info("client qos2 handle publish again", zap.String("client", client.MetaString()), zap.String("topic", topic),
				zap.Uint16("packetID", packet.PacketID))
		}
		return
	default:
		pubAck.ReasonCode = packets.PubackUnspecifiedError
	}
	broker.writePacket(client, pubAck)
}

func storePublishPacket(broker *Broker, client *client.Client, packet *packets.Publish) (string, error) {
	var (
		encodedData []byte
		err         error
		messageID   string
		topic       = packet.Topic
	)
	if encodedData, err = pkg.Encode(packet); err == nil {
		messageID, err = broker.store.CreatePacket(topic, encodedData)
		if err != nil {
			logger.Logger.Error("create packet to store error = ", zap.Error(err), zap.String("topic", topic), zap.String("client", client.MetaString()))
		} else {
			logger.Logger.Debug("create packet to store success", zap.String("topic", topic), zap.String("clientID", client.GetID()),
				zap.String("messageID", messageID))
			event2.GlobalEvent.EmitStoreMessage(topic, messageID)
		}
	}
	return messageID, err
}
