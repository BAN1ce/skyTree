package broker

import (
	"fmt"
	"github.com/BAN1ce/skyTree/inner/server"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"log"
	"net"
)

type Option func(*Core)

func WithUserAuth(auth UserAuth) Option {
	return func(core *Core) {
		core.userAuth = auth
	}
}

func WithSubTree(tree SubTree) Option {
	return func(core *Core) {
		core.subTree = tree
	}
}

type Core struct {
	clients  map[string]*Client
	server   *server.Server
	userAuth UserAuth
	subTree  SubTree
}

func NewCore(option ...Option) *Core {
	var (
		ip   = `127.0.0.1`
		port = 9222
		core = &Core{
			clients: make(map[string]*Client),
		}
	)
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Fatalln("resolve tcp addr error: ", err)
	}
	core.server = server.NewTCPServer(tcpAddr)
	for _, opt := range option {
		opt(core)
	}
	return core
}

func (c *Core) Start() error {
	if err := c.server.Start(); err != nil {
		return err
	}
	c.acceptConn()
	return nil
}

func (c *Core) acceptConn() {
	for {
		conn, ok := c.server.Conn()
		if !ok {
			logger.Logger.Info("server closed")
			return
		}
		client := newClient(conn)
		client.Run(c.HandleClient)
	}
}

func (c *Core) HandleClient(client *Client, packet *packets.ControlPacket) {
	// TODO : check MQTT version
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		c.handleConnect(client, packet.Content.(*packets.Connect))
	case packets.PUBLISH:
		c.handlePublish(client, packet.Content.(*packets.Publish))
	case packets.SUBSCRIBE:
		c.handleSubscribe(client, packet)
	}
}

func (c *Core) handleConnect(client *Client, packet *packets.Connect) {
	var (
		conAck = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	)
	client.user = packet.Properties.User

	if (*packet.Properties.RequestResponseInfo) == 0x00 {
		return
	}
	// TODO: retain , will ......

	if (packet.Properties.AuthMethod) != "" {
		logger.Logger.Debug("auth method = ", packet.Properties.AuthMethod)
		if c.userAuth == nil {
			logger.Logger.Error("user auth is nil")
			client.Close()
			return
		}
		c.userAuth(packet.Properties.AuthMethod, packet.Properties.AuthData)
	}
	// TODO: response client identifier when client identifier is empty
	conAck.ReasonCode = 0x00
	if _, err := conAck.WriteTo(client); err != nil {
		logger.Logger.Error("write to client error = ", err.Error())
		client.Close()
	}

}

func (c *Core) handlePublish(client *Client, packet *packets.Publish) {

}

func (c *Core) handleSubscribe(client *Client, packet *packets.ControlPacket) {

}
