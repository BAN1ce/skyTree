package broker

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/server"
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/middleware"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"log"
	"net"
	"sync"
	"time"
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
	ctx            context.Context
	server         *server.Server
	userAuth       middleware.UserAuth
	subTree        pkg.SubTree
	store          pkg.Store
	clientManager  *client.Manager
	sessionManager *session.Manager
	publishPool    *pool.PublishPool
	publishRetry   facade.RetrySchedule
	preMiddleware  map[byte][]middleware.PacketMiddleware
	handlers       *Handlers
	mux            sync.Mutex
}

func NewBroker(option ...Option) *Broker {
	var (
		ip     = `127.0.0.1`
		port   = config.GetServer().GetBrokerPort()
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
	b.ctx = ctx
	if err := b.server.Start(); err != nil {
		return err
	}
	b.acceptConn()
	return nil
}

func (b *Broker) acceptConn() {
	var wg sync.WaitGroup
	for {
		conn, ok := b.server.Conn()
		if !ok {
			logger.Logger.Info("server closed")
			return
		}
		newClient := client.NewClient(conn, client.WithStore(b.store), client.WithConfig(client.Config{
			WindowSize:       10,
			ReadStoreTimeout: 3 * time.Second,
		}))
		wg.Add(1)
		go func(c *client.Client) {
			c.Run(b.ctx, b)
			logger.Logger.Info("client closed", "client id = ", c.ID, "client point = ", &c)
			wg.Done()
			b.DeleteClient(c.ID)
		}(newClient)
	}
}

// ------------------------------------ handle client MQTT Packet ------------------------------------//

func (b *Broker) HandlePacket(client *client.Client, packet *packets.ControlPacket) {
	// TODO : check MQTT version
	if err := b.executePreMiddleware(client, packet); err != nil {
		return
	}
	logger.Logger.Debug("handle packet type = ", packet.PacketType(), " client id = ", client.ID,
		"packet", packet)
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

func (b *Broker) CreateClient(client *client.Client) {
	b.mux.Lock()
	b.clientManager.CreateClient(client)
	b.mux.Unlock()

}
func (b *Broker) DeleteClient(clientID string) {
	b.mux.Lock()
	defer b.mux.Unlock()
	if c, ok := b.clientManager.ReadClient(clientID); !ok {
		return
	} else {
		b.clientManager.DeleteClient(c)
	}
}
