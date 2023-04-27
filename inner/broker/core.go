package broker

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/server"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/BAN1ce/skyTree/pkg/util"
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

func WithClientManager(manager *client.Manager) Option {
	return func(core *Core) {
		core.clientManager = manager
	}
}

func WithSubTree(tree SubTree) Option {
	return func(core *Core) {
		core.subTree = tree
	}
}

type Core struct {
	server        *server.Server
	userAuth      UserAuth
	subTree       SubTree
	clientManager *client.Manager
	bytePool      *pool.BytePool
	publishPool   *pool.PublishPool
}

func NewCore(option ...Option) *Core {
	var (
		ip   = `127.0.0.1`
		port = 9222
		core = &Core{
			bytePool:    pool.NewBytePool(),
			publishPool: pool.NewPublishPool(),
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
func (c *Core) Name() string {
	return "broker"
}

func (c *Core) Start(ctx context.Context) error {
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
		client := client.NewClient(conn)
		client.Run(c)
	}
}
func (c *Core) ListenClientClose(client *client.Client) {
	c.clientManager.DeleteClient(client)
}

// ------------------------------------ handle client MQTT Packet ------------------------------------//

func (c *Core) HandlePacket(client *client.Client, packet *packets.ControlPacket) {
	// TODO : check MQTT version
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		c.handleConnect(client, packet.Content.(*packets.Connect))
	case packets.PUBLISH:
		c.handlePublish(client, packet.Content.(*packets.Publish))
	case packets.SUBSCRIBE:
		c.handleSubscribe(client, packet.Content.(*packets.Subscribe))
	}
}

func (c *Core) handleConnect(client *client.Client, packet *packets.Connect) {
	var (
		conAck = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
		id     = packet.ClientID
	)

	// FIXME: check poperties exists
	// client.user = packet.Properties.User
	// if (*packet.Properties.RequestResponseInfo) == 0x00 {
	// 	return
	// }
	// // TODO: retain , will ......
	//
	// if (packet.Properties.AuthMethod) != "" {
	// 	logger.Logger.Debug("auth method = ", packet.Properties.AuthMethod)
	// 	if c.userAuth == nil {
	// 		logger.Logger.Error("user auth is nil")
	// 		client.Close()
	// 		return
	// 	}
	// 	c.userAuth(packet.Properties.AuthMethod, packet.Properties.AuthData)
	// }
	// TODO: response client identifier when client identifier is empty
	conAck.ReasonCode = 0x00
	client.SetID(id)
	c.clientManager.CreateClient(client)

	if _, err := conAck.WriteTo(client); err != nil {
		logger.Logger.Error("write to client error = ", err.Error())
		client.Close()
	}
}

func (c *Core) handlePublish(client *client.Client, packet *packets.Publish) {
	var (
		topic = packet.Topic
		qos   = uint8(packet.QoS)
	)
	if qos > config.GetPubMaxQos() {
		c.disconnectClient(packets.DisconnectQoSNotSupported, client)
		return
	}
	// TODO : check dup flag with middleware
	for clientID, clientQos := range c.subTree.Match(topic) {
		pub := c.publishPool.Get()
		pub.QoS = uint8(clientQos)
		util.CpPublish(packet, pub)
		if _, err := c.clientManager.Write(clientID, pub); err != nil {
			logger.Logger.Error("write to client error = ", err.Error())
			client.Close()
		}
	}
}

// handleSubscribe
func (c *Core) handleSubscribe(client *client.Client, packet *packets.Subscribe) {
	c.subTree.CreateSub(client.ID, packet.Subscriptions)
	var (
		subAck = packets.NewControlPacket(packets.SUBACK).Content.(*packets.Suback)
	)
	subAck.PacketID = packet.PacketID
	for range packet.Subscriptions {
		subAck.Reasons = append(subAck.Reasons, 0x00)
	}
	c.writePacket(client, subAck)
}

func (c *Core) handleUnsubscribe(client *client.Client, packet *packets.Unsubscribe) {
	var (
		packetID = packet.PacketID
		unsubAck = packets.NewControlPacket(packets.UNSUBACK).Content.(*packets.Unsuback)
	)
	// TODO: check unsub result
	c.subTree.DeleteSub(client.ID, packet.Topics)
	for range packet.Topics {
		unsubAck.Reasons = append(unsubAck.Reasons, 0x00)
	}
	unsubAck.PacketID = packetID
	c.writePacket(client, unsubAck)
}

// disconnectClient with code
func (c *Core) disconnectClient(code byte, client *client.Client) {
	var (
		disconnect = packets.NewControlPacket(packets.DISCONNECT).Content.(*packets.Disconnect)
	)
	disconnect.ReasonCode = code
	if _, err := disconnect.WriteTo(client); err != nil {
		logger.Logger.Error("write to client error = ", err.Error())
		client.Close()
	}
}

// ----------------------------------------- support ---------------------------------------------------//
// writePacket for collect all error log
func (c *Core) writePacket(client *client.Client, packet packets.Packet) {
	_, err := packet.WriteTo(client)
	if err != nil {
		logger.Logger.Error("write to client error = ", err.Error())
		client.Close()
	}
}
