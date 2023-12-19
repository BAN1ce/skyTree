package client

import (
	"fmt"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	topic2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InnerHandler struct {
	client *Client
}

func newInnerHandler(client *Client) *InnerHandler {
	return &InnerHandler{
		client: client,
	}
}

func (i *InnerHandler) HandlePacket(client *Client, packet *packets.ControlPacket) error {
	var (
		err error
	)
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		connectPacket := packet.Content.(*packets.Connect)
		if err = i.HandleConnect(connectPacket); err != nil {
			logger.Logger.Warn("handle connect error: ", zap.Error(err), zap.String("client", client.MetaString()))
		}

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
	default:
		err = fmt.Errorf("unknown packet type = %d", packet.FixedHeader.Type)
		logger.Logger.Warn("handle packet error: ", zap.String("client", client.MetaString()), zap.String("packet", packet.String()))
	}
	return err
}

func (i *InnerHandler) HandleConnect(connectPacket *packets.Connect) error {
	i.client.mux.Lock()
	defer i.client.mux.Unlock()
	sessionConnectProp := session.NewConnectProperties(connectPacket.Properties)
	i.client.connectProperties = sessionConnectProp
	if err := i.client.options.session.SetConnectProperties(sessionConnectProp); err != nil {
		return err
	}
	i.recoverTopicFromSession()

	if !connectPacket.WillFlag {
		return nil
	}

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

	var conAck = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	conAck.ReasonCode = packets.ConnackSuccess
	return i.client.writePacket(conAck)
}

func (i *InnerHandler) HandleSub(subscribe *packets.Subscribe) error {
	c := i.client
	c.mux.Lock()
	defer c.mux.Unlock()
	var (
		subIdentifier int
		subAck        = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = subscribe.PacketID
	if subscribe != nil {
		// store topic's subIdentifier
		if tmp := subscribe.Properties.SubscriptionIdentifier; tmp != nil {
			subIdentifier = *tmp
			for _, t := range subscribe.Subscriptions {
				c.subIdentifier[t.Topic] = subIdentifier
			}
		}
	}
	// create topic instance
	_, noShareSubscribePacket := topic2.SplitShareAndNoShare(subscribe)
	var brokerTopics = map[string]topic2.Topic{}

	// handle simple sub
	for _, subOptions := range noShareSubscribePacket.Subscriptions {
		brokerTopics[subOptions.Topic] = topic.CreateTopic(i.client, &subOptions)
	}

	// client handle sub and create qos0,qos1,qos2 subOptions
	for topicName, t := range brokerTopics {
		c.topicManager.AddTopic(topicName, t)
	}

	for _, subOption := range subscribe.Subscriptions {
		subAck.Reasons = append(subAck.Reasons, subOption.QoS)
	}
	return i.client.writePacket(subAck)
}

func (i *InnerHandler) HandleUnsub(unsubscribe *packets.Unsubscribe) error {
	c := i.client
	c.mux.Lock()
	defer c.mux.Unlock()
	for _, topicName := range unsubscribe.Topics {
		c.options.session.DeleteSubTopic(topicName)
		c.topicManager.DeleteTopic(topicName)
	}
	var unsubAck = packets.NewControlPacket(packets.UNSUBACK).Content.(*packets.Unsuback)
	unsubAck.PacketID = unsubscribe.PacketID
	for range unsubscribe.Topics {
		unsubAck.Reasons = append(unsubAck.Reasons, packets.UnsubackSuccess)
	}
	return i.client.writePacket(unsubAck)
}

func (i *InnerHandler) HandlePubAck(pubAck *packets.Puback) error {
	c := i.client
	topicName := c.identifierIDTopic[pubAck.PacketID]
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
	topicName := c.identifierIDTopic[pubRec.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubRec packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRec.PacketID))
		return
	}
	c.topicManager.HandlePublishRec(topicName, pubRec)
}

func (i *InnerHandler) HandlePubComp(pubRel *packets.Pubcomp) {
	c := i.client
	topicName := c.identifierIDTopic[pubRel.PacketID]
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
		recoverTopics = map[string]topic2.Topic{}
		getSession    = i.client.options.session
	)
	for topicName, subOption := range getSession.ReadSubTopics() {
		logger.Logger.Debug("recover topic from getSession", zap.String("topic", topicName), zap.Any("subOption", subOption))
		unfinishedMessage := getSession.ReadTopicUnFinishedMessage(topicName)
		latestAckMessageID, _ := getSession.ReadTopicLatestPushedMessageID(topicName)
		topic := topic.CreateTopicFromSession(i.client, &packets.SubOptions{
			Topic:             topicName,
			QoS:               byte(subOption.QoS),
			NoLocal:           subOption.NoLocal,
			RetainAsPublished: subOption.RetainAsPublished,
		}, unfinishedMessage, latestAckMessageID)
		recoverTopics[topicName] = topic
	}
	for topicName, t := range recoverTopics {
		i.client.topicManager.AddTopic(topicName, t)
	}
}
