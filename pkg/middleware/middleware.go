package middleware

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type UserAuth = func(method string, authData []byte) bool

type ConnectHandle = func(client *client.Client, connect *packets.Connect) error

type DisConnectHandle = func(client *client.Client, disconnect *packets.Disconnect) error

type PacketMiddleware interface {
	Handle(client *client.Client, packet *packets.ControlPacket) error
}
