package client

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	topic2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/utils"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	pong = packets.NewControlPacket(packets.PINGRESP).Content.(*packets.Pingresp)
)

type InnerHandler struct {
	client *Client
}

func newClientHandler(client *Client) *InnerHandler {
	return &InnerHandler{
		client: client,
	}
}

func (i *InnerHandler) HandlePacket(ctx context.Context, packet *packets.ControlPacket, client *Client) error {
	var (
		err error
	)
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		connectPacket := packet.Content.(*packets.Connect)
		if err = i.HandleConnect(connectPacket); err != nil {
			logger.Logger.Warn("handle connect error: ", zap.Error(err), zap.String("client", client.MetaString()))
		}
	case packets.PUBLISH:
		// do nothing

	case packets.SUBSCRIBE:
		subscribePacket := packet.Content.(*packets.Subscribe)
		if err = i.HandleSub(subscribePacket); err != nil {
			logger.Logger.Warn("handle subscribe error: ", zap.Error(err), zap.String("client", client.MetaString()))
		}
	case packets.UNSUBSCRIBE:
		unsubscribePacket := packet.Content.(*packets.Unsubscribe)
		if err = i.HandleUnsub(unsubscribePacket); err != nil {
			logger.Logger.Warn("handle unsubscribe error: ", zap.Error(err), zap.String("client", client.MetaString()))
		}
	case packets.PUBACK:
		pubAckPacket := packet.Content.(*packets.Puback)
		if err = i.HandlePubAck(pubAckPacket); err != nil {
			logger.Logger.Warn("handle pubAck error: ", zap.Error(err), zap.String("client", client.MetaString()))
		}
	case packets.PUBREC:
		pubRecPacket := packet.Content.(*packets.Pubrec)
		i.HandlePubRec(pubRecPacket)
	case packets.PUBCOMP:
		pubCompPacket := packet.Content.(*packets.Pubcomp)
		i.HandlePubComp(pubCompPacket)
	case packets.PINGREQ:
		return i.client.WritePacket(pong)

	default:
		err = fmt.Errorf("unknown packet type = %d", packet.FixedHeader.Type)
		logger.Logger.Warn("handle packet error: ", zap.String("client", client.MetaString()), zap.String("packet", packet.String()))
	}
	return err
}

func (i *InnerHandler) HandleConnect(connectPacket *packets.Connect) error {
	logger.Logger.Debug("handle connect", zap.String("clientID", i.client.GetID()), zap.String("uid", i.client.GetUid()))
	i.client.mux.Lock()
	defer i.client.mux.Unlock()
	sessionConnectProp := session.NewConnectProperties(connectPacket.Properties)
	i.client.connectProperties = sessionConnectProp
	if err := i.client.component.session.SetConnectProperties(sessionConnectProp); err != nil {
		return err
	}
	i.recoverTopicFromSession()

	var (
		windowSize = -1
	)
	if connectPacket.Properties.ReceiveMaximum != nil && *connectPacket.Properties.ReceiveMaximum > 0 {
		windowSize = int(*connectPacket.Properties.ReceiveMaximum)
	}
	if windowSize == -1 {
		i.client.publishBucket = utils.NewBucket(windowSize)
	}
	if connectPacket.WillFlag {
		if err := i.client.setWill(&session.WillMessage{
			Topic:       connectPacket.WillTopic,
			QoS:         int(connectPacket.WillQOS),
			Property:    &session.WillProperties{Properties: connectPacket.WillProperties},
			Retain:      false,
			Payload:     connectPacket.WillMessage,
			DelayTaskID: uuid.NewString(),
		}); err != nil {
			return err
		}
	}

	var conAck = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	conAck.ReasonCode = packets.ConnackSuccess
	return i.client.writePacket(conAck)
}

func (i *InnerHandler) HandleSub(subscribe *packets.Subscribe) error {
	c := i.client
	c.mux.Lock()
	defer c.mux.Unlock()
	var (
		subAck = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = subscribe.PacketID

	// create topic instance
	_, noShareSubscribePacket := topic2.SplitShareAndNoShare(subscribe)
	var brokerTopics = map[string]topic2.Topic{}

	// handle simple sub
	for _, subOptions := range noShareSubscribePacket.Subscriptions {
		meta := topic2.NewMetaFromSubPacket(&subOptions, noShareSubscribePacket.Properties)
		brokerTopics[subOptions.Topic] = topic.CreateTopic(i.client, meta)
		i.client.getSession().CreateSubTopic(meta)
		subAck.Reasons = append(subAck.Reasons, subOptions.QoS)
	}

	// client handle sub and create qos0,qos1,qos2 subOptions
	for topicName, t := range brokerTopics {
		c.topicManager.AddTopic(topicName, t)

		// get retain message after sub
		if message, ok := c.component.retain.GetRetainMessage(topicName); ok {
			pub := packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
			pub.Payload = message.Payload
			pub.Topic = topicName
			if err := t.Publish(&packet.Message{
				PublishPacket: pub,
			}); err != nil {
				logger.Logger.Error("retain message publish error",
					zap.Error(err), zap.String("topic", topicName), zap.String("client", c.MetaString()))
			}
		}
	}

	return i.client.writePacket(subAck)
}

func (i *InnerHandler) HandleUnsub(unsubscribe *packets.Unsubscribe) error {
	c := i.client
	c.mux.Lock()
	defer c.mux.Unlock()

	var (
		unsubAck = packets.NewControlPacket(packets.UNSUBACK).Content.(*packets.Unsuback)
	)
	unsubAck.PacketID = unsubscribe.PacketID

	for _, topicName := range unsubscribe.Topics {
		c.component.session.DeleteSubTopic(topicName)
		c.topicManager.DeleteTopic(topicName)
		unsubAck.Reasons = append(unsubAck.Reasons, packets.UnsubackSuccess)
	}

	return i.client.writePacket(unsubAck)
}

func (i *InnerHandler) HandlePubAck(pubAck *packets.Puback) error {
	c := i.client
	topicName := c.packetIdentifierIDTopic[pubAck.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubAck packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubAck.PacketID))
		return nil
	}
	c.publishBucket.PutToken()
	c.topicManager.HandlePublishAck(topicName, pubAck)
	return nil
}

func (i *InnerHandler) HandlePubRec(pubRec *packets.Pubrec) {
	c := i.client
	topicName := c.packetIdentifierIDTopic[pubRec.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubRec packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRec.PacketID))
		return
	}
	c.topicManager.HandlePublishRec(topicName, pubRec)
}

func (i *InnerHandler) HandlePubComp(pubRel *packets.Pubcomp) {
	c := i.client
	topicName := c.packetIdentifierIDTopic[pubRel.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubComp packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRel.PacketID))
		return
	}
	c.publishBucket.PutToken()
	c.topicManager.HandelPublishComp(topicName, pubRel)
}

var (
	// use the same ping resp packet
	pingResp = packets.NewControlPacket(packets.PINGRESP).Content.(*packets.Pingresp)
)

func (i *InnerHandler) HandlePing(_ *packets.Pingreq) {
	i.client.RefreshAliveTime()
	_ = i.client.WritePacket(pingResp)
}

func (i *InnerHandler) recoverTopicFromSession() {
	var (
		getSession = i.client.component.session
	)
	for _, topicMeta := range getSession.ReadSubTopics() {
		var (
			topicName = topicMeta.Topic
		)
		logger.Logger.Debug("recover topic from getSession", zap.Any("meta", topicMeta))
		unfinishedMessage := getSession.ReadTopicUnFinishedMessage(topicName)
		subTopic := topic.CreateTopicFromSession(i.client, &topicMeta, unfinishedMessage)
		i.client.topicManager.AddTopic(topicName, subTopic)
	}
}
