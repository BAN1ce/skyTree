package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/retain"
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
			client.SetClientTopicAlias(packet.Topic, *(packet.Properties.TopicAlias))
			return nil
		}
		// get topic by topic alias
		packet.Topic = client.GetClientTopicAlias(*(packet.Properties.TopicAlias))
		if packet.Topic == "" {
			return errs.ErrTopicAliasNotFound
		}
		return nil
	}
	return errs.ErrTopicAliasInvalid
}

func (p *PublishHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) error {
	var (
		packet     = rawPacket.Content.(*packets.Publish)
		qos        = packet.QoS
		err        error
		messageID  string
		reasonCode byte
	)

	// response puback or pubrec at the end of this function
	defer func() {
		p.response(client, reasonCode, packet)
	}()

	// handle topic alias, if topic alias is not 0, then use topic alias
	// if topic alias is 0, then use topic
	if err = p.handleTopicAlias(packet, client); err != nil {
		logger.Logger.Error("handle topic alias error", zap.Error(err))
		reasonCode = packets.PubackUnspecifiedError
		p.response(client, reasonCode, packet)
		return err
	}

	var (
		topic          = packet.Topic
		subTopics      = broker.subTree.MatchTopic(topic)
		publishMessage = &packet2.Message{
			ClientID:      client.GetID(),
			PublishPacket: packet,
		}
	)

	// double check topic name
	if topic == "" {
		reasonCode = packets.PubackTopicNameInvalid
		return err
	}

	if len(packet.Payload) == 0 {
		if packet.Retain {
			if err = broker.state.DeleteRetainMessageID(topic); err != nil {
				reasonCode = packets.PubackUnspecifiedError
				return err
			}
		} else {
			reasonCode = packets.PubackUnspecifiedError
			return err
		}
	}

	if len(subTopics) == 0 {
		reasonCode = packets.PubackNoMatchingSubscribers
		return err
	}

	// TODO: should emit all wildcard messageStore
	// TODO: should emit all wildcard messageStore
	event.GlobalEvent.EmitClientPublish(topic, publishMessage)

	switch qos {
	case broker2.QoS0:
		if messageID, err = broker.messageStore.StorePublishPacket(subTopics, publishMessage); err != nil {
			logger.Logger.Error("messageStore publish packet for QoS0 error", zap.Error(err), zap.String("messageStore", topic), zap.String("messageID", messageID))
		}
	case broker2.QoS1:
		if len(subTopics) == 0 && !packet.Retain {
			reasonCode = packets.PubackNoMatchingSubscribers
			return err
		}
		if len(subTopics) == 0 {
			subTopics = map[string]int32{
				topic: int32(packet.QoS),
			}
		}
		// messageStore message
		if messageID, err = broker.messageStore.StorePublishPacket(subTopics, publishMessage); err != nil {
			logger.Logger.Error("messageStore publish packet error", zap.Error(err), zap.String("messageStore", topic), zap.String("messageID", messageID))
			reasonCode = packets.PubackUnspecifiedError
		} else {
			reasonCode = packets.PubackSuccess
		}
	case broker2.QoS2:
		pubrec := packet2.NewPublishRec()
		if len(subTopics) == 0 {
			pubrec.ReasonCode = packets.PubrecNoMatchingSubscribers
			return err
		}
		if !client.QoS2.HandlePublish(packet) {
			pubrec.ReasonCode = packets.PubrecUnspecifiedError
			return err
		}
	default:
		reasonCode = packets.PubackUnspecifiedError
		return err
	}

	if packet.Retain {
		// if payload is empty, then delete retain message
		if err = broker.retain.PutRetainMessage(&retain.Message{
			Topic:   topic,
			Payload: packet.Payload,
		}); err != nil {
			reasonCode = packets.PubackUnspecifiedError
		}
	}
	return err
}

func (p *PublishHandler) response(client *client.Client, reasonCode byte, publish *packets.Publish) {
	switch publish.QoS {
	case broker2.QoS1:
		pubAck := packet2.NewPublishAck()
		pubAck.ReasonCode = reasonCode
		pubAck.PacketID = publish.PacketID
		client.WritePacket(pubAck)
	case broker2.QoS2:
		pubRec := packet2.NewPublishRec()
		pubRec.ReasonCode = reasonCode
		pubRec.PacketID = publish.PacketID
		client.WritePacket(pubRec)
	case broker2.QoS0:
		// do nothing
	default:
		logger.Logger.Error("wrong qos", zap.Uint8("qos", publish.QoS))
	}

}
