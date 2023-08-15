package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/errs"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type PublishHandler struct {
}

func NewPublishHandler() *PublishHandler {
	return &PublishHandler{}
}

func (p *PublishHandler) handleTopicAlias(packet *packets.Publish, client *client.Client) error {
	if packet.Properties == nil {
		return nil
	}
	if packet.Properties.TopicAlias == nil {
		return nil
	}
	if alias := *(packet.Properties.TopicAlias); alias != 0 {
		if packet.Topic != "" {
			client.SetTopicAlias(packet.Topic, *(packet.Properties.TopicAlias))
			return nil
		}
		packet.Topic = client.GetTopicAlias(*(packet.Properties.TopicAlias))
		if packet.Topic == "" {
			return errs.ErrTopicAliasNotFound
		}
		return nil
	}
	return errs.ErrTopicAliasInvalid
}

func (p *PublishHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet     = rawPacket.Content.(*packets.Publish)
		topic      = packet.Topic
		qos        = packet.QoS
		pubAck     = packet2.NewPublishAck()
		subClients = broker.subTree.Match(topic)
		err        error
	)
	pubAck.PacketID = packet.PacketID
	if err = p.handleTopicAlias(packet, client); err != nil {
		logger.Logger.Error("handle topic alias error", zap.Error(err))
		pubAck.ReasonCode = packets.PubackUnspecifiedError
		broker.writePacket(client, pubAck)
		return
	}
	// TODO: should emit all wildcard store
	event.Event.Emit(event.ClientPublish, topic)
	event.Event.Emit(event.ReceivedTopicPublishEventName(topic), topic, packet)

	switch qos {
	case broker.QoS0:
		return
	case broker.QoS1:
		if len(subClients) == 0 {
			pubAck.ReasonCode = packets.PubackNoMatchingSubscribers
			broker.writePacket(client, pubAck)
			return
		}
		// store message
		if _, err = broker.store.StorePublishPacket(packet); err != nil {
			pubAck.ReasonCode = packets.PubackUnspecifiedError
		} else {
			pubAck.ReasonCode = packets.PubackSuccess
		}
		client.WritePacket(pubAck)
	case broker.QoS2:
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
		}
		logger.Logger.Info("client qos2 handle publish again", zap.String("client", client.MetaString()), zap.String("store", topic),
			zap.Uint16("packetID", packet.PacketID))
		return
	default:
		pubAck.ReasonCode = packets.PubackUnspecifiedError
	}

}
