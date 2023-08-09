package broker

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/inner/broker/server"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/middleware"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"log"
	"net"
	"sync"
	"time"
)

type Handlers struct {
	Connect     brokerHandler
	Publish     brokerHandler
	PublishAck  brokerHandler
	PublishRec  brokerHandler
	PublishRel  brokerHandler
	PublishComp brokerHandler
	Ping        brokerHandler
	Sub         brokerHandler
	UnSub       brokerHandler
	Auth        brokerHandler
	Disconnect  brokerHandler
}

type brokerHandler interface {
	Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket)
}
type Broker struct {
	ctx            context.Context
	server         *server.Server
	userAuth       middleware.UserAuth
	subTree        pkg.SubTree
	store          *store.Wrapper
	clientManager  *ClientManager
	sessionManager pkg.SessionManager
	publishPool    *pool.Publish
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
			publishPool:   pool.NewPublish(),
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
	return "store"
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
		}), client.WithNotifyClose(b))
		wg.Add(1)
		go func(c *client.Client) {
			c.Run(b.ctx, b)
			logger.Logger.Info("client closed", zap.String("client", c.MetaString()))
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
	logger.Logger.Debug("handle packet", zap.String("client", client.MetaString()), zap.Any("packet", packet))
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		event.Event.Emit(event.Connect)
		b.handlers.Connect.Handle(b, client, packet)
	case packets.PUBLISH:
		b.handlers.Publish.Handle(b, client, packet)

	case packets.PUBACK:
		event.Event.Emit(event.ClientPublishAck)
		b.handlers.PublishAck.Handle(b, client, packet)
	case packets.PUBREC:
		b.handlers.PublishRec.Handle(b, client, packet)
	case packets.PUBREL:
		b.handlers.PublishRel.Handle(b, client, packet)
	case packets.PUBCOMP:
		b.handlers.PublishComp.Handle(b, client, packet)

	case packets.SUBSCRIBE:
		event.Event.Emit(event.Subscribe)
		b.handlers.Sub.Handle(b, client, packet)
	case packets.UNSUBSCRIBE:
		event.Event.Emit(event.Unsubscribe)
		b.handlers.UnSub.Handle(b, client, packet)
	case packets.PINGREQ:
		event.Event.Emit(event.Ping)
		b.handlers.Ping.Handle(b, client, packet)
	case packets.DISCONNECT:
		event.Event.Emit(event.Disconnect)
		b.handlers.Disconnect.Handle(b, client, packet)
	case packets.AUTH:
		event.Event.Emit(event.ClientAuth)
		b.handlers.Auth.Handle(b, client, packet)
	default:
		logger.Logger.Warn("unknown packet type = ", zap.Uint8("type", packet.FixedHeader.Type))
	}
}

func (b *Broker) executePreMiddleware(client *client.Client, packet *packets.ControlPacket) error {
	for _, midHandle := range b.preMiddleware[packet.FixedHeader.Type] {
		if err := midHandle.Handle(client, packet); err != nil {
			logger.Logger.Error("middleware handle error: ", zap.Error(err), zap.String("client", client.MetaString()))
			client.Close()
			return err
		}
	}
	return nil
}

// ----------------------------------------- support ---------------------------------------------------//
// writePacket for collect all error log
func (b *Broker) writePacket(client *client.Client, packet packets.Packet) {
	client.WritePacket(packet)
}

func (b *Broker) CreateClient(client *client.Client) {
	b.mux.Lock()
	b.clientManager.CreateClient(client)
	b.mux.Unlock()

}
func (b *Broker) DeleteClient(clientID string) {
	b.mux.Lock()
	b.clientManager.DeleteClient(clientID)
	b.mux.Unlock()
}

func (b *Broker) NotifyClientClose(client *client.Client) {
	b.mux.Lock()
	b.clientManager.deleteClient(client)
	b.mux.Unlock()
}
