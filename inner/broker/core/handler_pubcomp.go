package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type PublishComp struct {
}

func NewPublishComp() *PublishComp {
	return &PublishComp{}
}

func (a *PublishComp) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	// receive publish comp packet, then client delete publish packet from qos2
	return nil
}
