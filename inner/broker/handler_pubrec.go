package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type PubRec struct {
}

func (p *PubRec) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) {
	// TODO implement me
	panic("implement me")
}
