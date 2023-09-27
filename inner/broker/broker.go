package broker

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/server"
	"github.com/BAN1ce/skyTree/inner/broker/state"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/middleware"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
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
	state          *state.State
	userAuth       middleware.UserAuth
	subTree        broker.SubCenter
	store          *store.Wrapper
	clientManager  *ClientManager
	sessionManager broker.SessionManager
	publishPool    *pool.Publish
	publishRetry   facade.RetrySchedule
	preMiddleware  map[byte][]middleware.PacketMiddleware
	handlers       *Handlers
	mux            sync.Mutex
}

func NewBroker(option ...Option) *Broker {
	var (
		ip   = `127.0.0.1`
		port = config.GetServer().GetBrokerPort()
		b    = &Broker{
			publishPool:   pool.NewPublish(),
			preMiddleware: make(map[byte][]middleware.PacketMiddleware),
		}
	)
	b.initMiddleware()
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Fatalln("resolve tcp addr error: ", err)
	}
	b.server = server.NewTCPServer(tcpAddr)
	for _, opt := range option {
		opt(b)
	}

	return b
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
		newClient := client.NewClient(conn, client.WithConfig(client.Config{
			WindowSize:       10,
			ReadStoreTimeout: 3 * time.Second,
		}), client.WithNotifyClose(b))
		wg.Add(1)
		go func(c *client.Client) {
			c.Run(b.ctx, b)
			logger.Logger.Info("client closed", zap.String("client", c.MetaString()))
			wg.Done()
		}(newClient)
	}
}

// ------------------------------------ handle client MQTT PublishPacket ------------------------------------//

func (b *Broker) HandlePacket(client *client.Client, packet *packets.ControlPacket) {
	// TODO : check MQTT version
	if err := b.executePreMiddleware(client, packet); err != nil {
		return
	}
	logger.Logger.Debug("handle packet", zap.String("client", client.MetaString()), zap.Any("packet", packet))
	switch packet.FixedHeader.Type {
	case packets.CONNECT:
		b.handlers.Connect.Handle(b, client, packet)
	case packets.PUBLISH:
		b.handlers.Publish.Handle(b, client, packet)

	case packets.PUBACK:
		b.handlers.PublishAck.Handle(b, client, packet)
	case packets.PUBREC:
		b.handlers.PublishRec.Handle(b, client, packet)
	case packets.PUBREL:
		b.handlers.PublishRel.Handle(b, client, packet)
	case packets.PUBCOMP:
		b.handlers.PublishComp.Handle(b, client, packet)

	case packets.SUBSCRIBE:
		b.handlers.Sub.Handle(b, client, packet)
	case packets.UNSUBSCRIBE:
		b.handlers.UnSub.Handle(b, client, packet)
	case packets.PINGREQ:
		b.handlers.Ping.Handle(b, client, packet)
	case packets.DISCONNECT:
		b.handlers.Disconnect.Handle(b, client, packet)
	case packets.AUTH:
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

func (b *Broker) NotifyWillMessage(message *broker.WillMessage) {
	var (
		willPublishMessage *packet2.PublishMessage
		ctx, cancel        = context.WithTimeout(b.ctx, 3*time.Second)
	)
	defer cancel()
	if err := store.ReadTopicWillMessage(ctx, message.Topic, message.MessageID, func(message *packet2.PublishMessage) {
		willPublishMessage = message
	}); err != nil {
		logger.Logger.Error("read will message error", zap.Error(err))
		return
	}
	if willPublishMessage == nil {
		logger.Logger.Warn("read will message is nil", zap.String("topic", message.Topic), zap.String("messageID", message.MessageID))
		return
	}
	logger.Logger.Debug("read will message", zap.String("topic", message.Topic),
		zap.String("messageID", message.MessageID), zap.String("publishMessageID", willPublishMessage.MessageID))

	// check will message expired, if expired, delete will message from store and return
	// delete will message from store if retain is false
	if !message.Retain {
		if err := store.DeleteTopicMessageID(context.TODO(), message.Topic, message.MessageID); err != nil {
			logger.Logger.Error("delete will message error", zap.Error(err), zap.String("topic", message.Topic), zap.String("messageID", message.MessageID))
		}
		logger.Logger.Debug("delete will message", zap.String("topic", message.Topic), zap.String("messageID", message.MessageID))
	}
	clients := b.subTree.Match(message.Topic)
	pubMessage := message.ToPublishPacket()
	pubMessage.Payload = willPublishMessage.PublishPacket.Payload
	for id := range clients {
		b.clientManager.Write(id, pubMessage)
	}
}

func (b *Broker) ReadTopicRetainWillMessage(topic string) []*packet2.PublishMessage {
	var (
		willPublishMessage []*packet2.PublishMessage
		messageID, err     = b.state.ReadTopicWillMessageID(topic)
		ctx, cancel        = context.WithCancel(b.ctx)
	)
	cancel()
	if err != nil {
		logger.Logger.Error("read will message error", zap.Error(err), zap.String("topic", topic))
		return nil
	}
	for id, clientID := range messageID {
		// skip online client's will message
		if _, online := b.clientManager.ReadClient(clientID); online {
			continue
		}
		if err := store.ReadPublishMessage(ctx, topic, id, 1, true, func(message *packet2.PublishMessage) {
			willPublishMessage = append(willPublishMessage, message)
		}); err != nil {
			logger.Logger.Error("read will message error", zap.Error(err), zap.String("topic", topic), zap.String("messageID", id))
		}
	}
	return willPublishMessage
}

func (b *Broker) ReadTopicRetainMessage(topic string) []*packet2.PublishMessage {
	var (
		retainMessage  []*packet2.PublishMessage
		messageID, err = b.state.ReadRetainMessageID(topic)
		ctx, cancel    = context.WithCancel(b.ctx)
	)
	cancel()
	if err != nil {
		logger.Logger.Error("read retain message error", zap.Error(err), zap.String("topic", topic))
		return nil
	}
	for _, id := range messageID {
		if err := store.ReadPublishMessage(ctx, topic, id, 1, true, func(message *packet2.PublishMessage) {
			retainMessage = append(retainMessage, message)
		}); err != nil {
			logger.Logger.Error("read retain message error", zap.Error(err), zap.String("topic", topic), zap.String("messageID", id))
		}
	}
	return retainMessage

}
func (b *Broker) ReleaseSession(clientID string) {
	var (
		session, ok = b.sessionManager.ReadSession(clientID)
	)
	if !ok {
		return
	}
	// release will message info from broker state machine
	b.ReleaseWillMessage(session)
	// release session from session store
	session.Release()
}

// ReleaseWillMessage release will message from broker state machine and message store
func (b *Broker) ReleaseWillMessage(session broker.Session) {
	willMessage, err := session.GetWillMessage()
	if err != nil {
		logger.Logger.Error("release session get will message error", zap.Error(err))
		return
	}

	// delete will message delay task if will message has delay task
	if willMessage.Property.WillDelayInterval != 0 {
		facade.GetWillDelay().Delete(willMessage.DelayTaskID)
		logger.Logger.Debug("release session delete will message delay task", zap.String("delayTaskID", willMessage.DelayTaskID))
	}

	logger.Logger.Debug("release session delete will message from message store", zap.String("topic", willMessage.Topic),
		zap.String("messageID", willMessage.MessageID))
	// delete will message from message store
	if err := store.DeleteTopicMessageID(context.TODO(), willMessage.Topic, willMessage.MessageID); err != nil {
		logger.Logger.Error("release session delete will message from message store error", zap.Error(err))
	}

	logger.Logger.Debug("release session delete will message from state", zap.String("topic", willMessage.Topic), zap.String("messageID", willMessage.MessageID))
	// delete will message ID from state
	if err := b.state.DeleteTopicWillMessageID(willMessage.Topic, willMessage.MessageID); err != nil {
		logger.Logger.Error("release session delete will message from state error", zap.Error(err))
	}

}
