package broker

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/server"
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/middleware"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"log"
	"net"
	"sync"
)

type Handlers struct {
	Connect    brokerHandler
	Publish    brokerHandler
	Ping       brokerHandler
	Sub        brokerHandler
	UnSub      brokerHandler
	Auth       brokerHandler
	Disconnect brokerHandler
}

type brokerHandler interface {
	Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket)
}
type Broker struct {
	mux            sync.RWMutex
	server         *server.Server
	userAuth       middleware.UserAuth
	subTree        pkg.SubTree
	clientManager  *client.Manager
	sessionManager *session.Manager
	publishPool    *pool.PublishPool
	preMiddleware  map[byte][]middleware.PacketMiddleware
	handlers       *Handlers
}

func NewBroker(option ...Option) *Broker {
	var (
		ip     = `127.0.0.1`
		port   = 9222
		broker = &Broker{
			publishPool:   pool.NewPublishPool(),
			preMiddleware: make(map[byte][]middleware.PacketMiddleware),
		}
	)
	broker.initMiddleware()
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Fatalln("resolve tcp addr error: ", err)
	}
	broker.server = server.NewTCPServer(tcpAddr)
	for _, opt := range option {
		opt(broker)
	}

	return broker
}

func (b *Broker) Name() string {
	return "broker"
}

func (b *Broker) Start(ctx context.Context) error {
	if err := b.server.Start(); err != nil {
		return err
	}
	b.acceptConn()
	return nil
}

func (b *Broker) acceptConn() {
	for {
		conn, ok := b.server.Conn()
		if !ok {
			logger.Logger.Info("server closed")
			return
		}
		newClient := client.NewClient(conn)
		newClient.Run(b)
	}
}
func (b *Broker) ListenClientClose(client *client.Client) {
	b.clientManager.DeleteClient(client)
}

// ------------------------------------ handle client MQTT Packet ------------------------------------//

func (b *Broker) HandlePacket(client *client.Client, packet *packets.ControlPacket) {
	// TODO : check MQTT version
	if err := b.executePreMiddleware(client, packet); err != nil {
		return
	}
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		b.handlers.Connect.Handle(b, client, packet)
	case packets.PUBLISH:
		b.handlers.Publish.Handle(b, client, packet)
	case packets.SUBSCRIBE:
		b.handlers.Sub.Handle(b, client, packet)
	case packets.UNSUBSCRIBE:
		b.handlers.UnSub.Handle(b, client, packet)
	case packets.PINGREQ:
		b.handlers.Ping.Handle(b, client, packet)
	}
}

func (b *Broker) executePreMiddleware(client *client.Client, packet *packets.ControlPacket) error {
	for _, midHandle := range b.preMiddleware[packet.FixedHeader.Type] {
		if err := midHandle.Handle(client, packet); err != nil {
			logger.Logger.Error("handle connect error: ", err)
			client.Close()
			return err
		}
	}
	return nil
}

// disconnectClient with code
func (b *Broker) disconnectClient(code byte, client *client.Client) {
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
func (b *Broker) writePacket(client *client.Client, packet packets.Packet) {
	// TODO: check client property setting
	_, err := packet.WriteTo(client)
	if err != nil {
		logger.Logger.Error("write to client error = ", err.Error())
		client.Close()
	}
}
