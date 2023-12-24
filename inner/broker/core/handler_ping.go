package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/pkg/utils"
	"github.com/eclipse/paho.golang/packets"
)

type PingHandler struct {
}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (p *PingHandler) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	return broker.clientKeepAliveMonitor.SetClientAliveTime(client.UID, utils.NextAliveTime(int64(client.GetKeepAliveTime().Seconds())))
}
