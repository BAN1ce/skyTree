package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type DisconnectHandler struct {
}

func NewDisconnectHandler() *DisconnectHandler {
	return &DisconnectHandler{}
}

func (d *DisconnectHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	// var (
	// 		packet = rawPacket.Content.(*packets.Disconnect)
	// 	)
}
