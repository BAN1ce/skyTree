package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type PubAck struct {
}

func (a *PubAck) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	// TODO implement me
	panic("implement me")
}
