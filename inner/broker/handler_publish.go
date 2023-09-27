package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
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
		packet    = rawPacket.Content.(*packets.Publish)
		qos       = packet.QoS
		pubAck    = packet2.NewPublishAck()
		err       error
		messageID string
	)
	pubAck.PacketID = packet.PacketID
	// handle topic alias, if topic alias is not 0, then use topic alias
	// if topic alias is 0, then use topic
	if err = p.handleTopicAlias(packet, client); err != nil {
		logger.Logger.Error("handle topic alias error", zap.Error(err))
		pubAck.ReasonCode = packets.PubackUnspecifiedError
		broker.writePacket(client, pubAck)
		return
	}
	var (
		topic          = packet.Topic
		subTopics      = broker.subTree.MatchTopic(topic)
		publishMessage = &packet2.PublishMessage{
			ClientID:      client.GetID(),
			PublishPacket: packet,
		}
	)
	defer func() {
		broker.writePacket(client, pubAck)
	}()

	// double check topic name
	if topic == "" {
		pubAck.ReasonCode = packets.PubackTopicNameInvalid
		return
	}

	if len(packet.Payload) == 0 {
		if packet.Retain {
			if err = broker.state.DeleteRetainMessageID(topic); err != nil {
				pubAck.ReasonCode = packets.PubackUnspecifiedError
				return
			}
		} else {
			pubAck.ReasonCode = packets.PubackUnspecifiedError
			return
		}
	}

	defer func() {
		if packet.Retain && messageID != "" {
			if err = broker.state.CreateRetainMessageID(packet.Topic, messageID); err != nil {
				pubAck.ReasonCode = packets.PubackUnspecifiedError
				return
			}
		}
	}()

	// TODO: should emit all wildcard store
	// TODO: should emit all wildcard store
	event.GlobalEvent.EmitClientPublish(topic, publishMessage)

	switch qos {
	case broker2.QoS0:
		if !packet.Retain {
			return
		}

	case broker2.QoS1:

		if len(subTopics) == 0 && !packet.Retain {
			pubAck.ReasonCode = packets.PubackNoMatchingSubscribers
			return
		}
		if len(subTopics) == 0 {
			subTopics = map[string]int32{
				topic: int32(packet.QoS),
			}
		}
		// store message
		if messageID, err = broker.store.StorePublishPacket(subTopics, publishMessage); err != nil {
			logger.Logger.Error("store publish packet error", zap.Error(err), zap.String("store", topic))
			pubAck.ReasonCode = packets.PubackUnspecifiedError
		} else {
			pubAck.ReasonCode = packets.PubackSuccess
		}

	case broker2.QoS2:
		pubrec := packet2.NewPublishRec()
		if len(subTopics) == 0 {
			pubrec.ReasonCode = packets.PubrecNoMatchingSubscribers
			return
		}
		if client.QoS2.HandlePublish(packet) {
			pubrec.PacketID = packet.PacketID
			return
		}
		logger.Logger.Info("client qos2 handle publish again", zap.String("client", client.MetaString()), zap.String("store", topic),
			zap.Uint16("packetID", packet.PacketID))
		return

	default:
		pubAck.ReasonCode = packets.PubackUnspecifiedError
		return
	}

}
