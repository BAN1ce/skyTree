package broker

import (
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/util"
	"github.com/eclipse/paho.golang/packets"
)

type PublishHandler struct {
}

func NewPublishHandler() *PublishHandler {
	return &PublishHandler{}
}

func (p *PublishHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	var (
		packet = rawPacket.Content.(*packets.Publish)
		topic  = packet.Topic
		qos    = uint8(packet.QoS)
	)
	if qos > config.GetPubMaxQos() {
		broker.disconnectClient(packets.DisconnectQoSNotSupported, client)
		return
	}
	// TODO : check dup flag with middleware
	for clientID, clientQos := range broker.subTree.Match(topic) {
		pub := broker.publishPool.Get()
		pub.QoS = uint8(clientQos)
		util.CpPublish(packet, pub)
		if _, err := broker.clientManager.Write(clientID, pub); err != nil {
			logger.Logger.Error("write to client error = ", err.Error())
			client.Close()
		}
	}
}
