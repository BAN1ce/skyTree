package core

import (
	middleware2 "github.com/BAN1ce/skyTree/pkg/middleware"
	"github.com/eclipse/paho.golang/packets"
)

func (b *Broker) initMiddleware() {
	b.initConnectMiddleware()
	b.initPublishMiddleware()
}

func (b *Broker) initConnectMiddleware() {
	b.preMiddleware[packets.CONNECT] = []middleware2.PacketMiddleware{
		&middleware2.ProtocolVersion{},
		&middleware2.OnceConnect{},
	}
}

func (b *Broker) initPublishMiddleware() {
	b.preMiddleware[packets.PUBLISH] = []middleware2.PacketMiddleware{}
}
