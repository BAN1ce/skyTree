package client

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/BAN1ce/skyTree/pkg/util"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"net"
	"strings"
	"sync"
	"time"
)

type PacketIDFactory interface {
	Generate() uint16
}

type Config struct {
	WindowSize       int
	ReadStoreTimeout time.Duration
}

type Handler interface {
	HandlePacket(*Client, *packets.ControlPacket)
}

type Client struct {
	ID                string `json:"id"`
	connectProperties *packet.ConnectProperties
	ctx               context.Context
	mux               sync.RWMutex
	conn              net.Conn
	handler           Handler
	state             state.State
	options           *Options
	packetIDFactory   PacketIDFactory
	publishBucket     *util.Bucket
	messages          chan pkg.Message
	topics            *topic.Topics
	identifierIDTopic map[uint16]string
	QoS2              *HandleQoS2
	topicAlias        map[uint16]string
}

func NewClient(conn net.Conn, option ...Option) *Client {
	var (
		c = &Client{
			conn:              conn,
			options:           new(Options),
			identifierIDTopic: map[uint16]string{},
			topicAlias:        map[uint16]string{},
		}
	)
	for _, o := range option {
		o(c.options)
	}
	c.packetIDFactory = util.NewPacketIDFactory()
	c.messages = make(chan pkg.Message, c.options.cfg.WindowSize)
	c.publishBucket = util.NewBucket(c.options.cfg.WindowSize)
	c.QoS2 = NewHandleQoS2()

	return c
}

func (c *Client) Run(ctx context.Context, handler Handler) {
	c.ctx = ctx
	c.handler = handler
	var (
		controlPacket *packets.ControlPacket
		err           error
	)
	for {
		controlPacket, err = packets.ReadPacket(c.conn)
		if err != nil {
			logger.Logger.Info("read controlPacket error = ", zap.Error(err), zap.String("client", c.MetaString()))
			c.closeHandleError()
			return
		}
		handler.HandlePacket(c, controlPacket)
	}
}

func (c *Client) HandleSub(subscribe *packets.Subscribe) map[string]byte {
	var (
		topics = subscribe.Subscriptions
		failed = map[string]byte{}
	)
	for t, v := range topics {
		// FIXME qos 和其它配置
		c.topics.CreateTopic(t, v.QoS)
		failed[t] = v.QoS
	}
	return failed
}

func (c *Client) HandleUnSub(topicName string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.options.session.DeleteSubTopic(topicName)
	c.topics.DeleteTopic(topicName)
}

func (c *Client) HandlePubAck(pubAck *packets.Puback) {
	topicName := c.identifierIDTopic[pubAck.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubAck packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubAck.PacketID))
		return
	}
	c.topics.HandlePublishAck(topicName, pubAck)
}

func (c *Client) HandlePubRec(pubRec *packets.Pubrec) {
	topicName := c.identifierIDTopic[pubRec.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubRec packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRec.PacketID))
		return
	}
	c.topics.HandlePublishRec(topicName, pubRec)
}

func (c *Client) HandlePubComp(pubRel *packets.Pubcomp) {
	topicName := c.identifierIDTopic[pubRel.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubComp packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRel.PacketID))
		return
	}
	c.topics.HandelPublishComp(topicName, pubRel)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) closeHandleError() {
	logger.Logger.Info("client close", zap.String("clientID", c.ID))
	if err := c.conn.Close(); err != nil {
		logger.Logger.Warn("close conn error", zap.Error(err), zap.String("client", c.MetaString()))
	}
	if err := c.topics.Close(); err != nil {
		logger.Logger.Warn("close topics error", zap.Error(err), zap.String("client", c.MetaString()))
	}
	// TODO: check will message
	c.options.notifyClose.NotifyClientClose(c)
}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}

func (c *Client) WritePacket(packet packets.Packet) {
	if packet == nil {
		return
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	logger.Logger.Debug("write packet", zap.String("client", c.MetaString()), zap.Any("packet", packet))
	c.writePacket(packet)
}
func (c *Client) Write(data []byte) error {
	// TODO: check size
	_, err := c.conn.Write(data)
	return err
}
func (c *Client) writePacket(packet packets.Packet) {
	var (
		buf              = pool.ByteBufferPool.Get()
		prepareWriteSize int64
		err              error
		topicName        string
	)
	// publishAck, subscribeAck, unsubscribeAck should use the same packetID as the original packet

	switch p := packet.(type) {
	case *packets.Publish:
		p.PacketID = c.packetIDFactory.Generate()
		logger.Logger.Debug("publish to client", zap.Uint16("packetID", p.PacketID), zap.String("client", c.MetaString()), zap.String("store", p.Topic))
		c.identifierIDTopic[p.PacketID] = p.Topic
		topicName = p.Topic
	}
	defer pool.ByteBufferPool.Put(buf)
	if prepareWriteSize, err = packet.WriteTo(buf); err != nil {
		logger.Logger.Info("write packet error", zap.Error(err), zap.String("client", c.MetaString()))
		// TODO: check maximum packet size should close client ?
	}
	// TODO: check maximum packet size
	if _, err = c.conn.Write(buf.Bytes()); err != nil {
		logger.Logger.Info("write packet error", zap.Error(err), zap.String("client", c.MetaString()), zap.Int64("prepareWriteSize", prepareWriteSize), zap.String("topicName", topicName))
		// TODO: check maximum packet size should close client ?
	}
}

func (c *Client) SetSession(session pkg.Session) error {
	c.mux.Lock()
	c.options.session = session
	c.topics = topic.NewTopicWithSession(c.ctx, c.options.session, topic.WithStore(c.options.Store), topic.WithWriter(c))
	c.mux.Unlock()
	return nil
}

func (c *Client) SetWill() {

}

func (c *Client) SetTopicAlias(topic string, alias uint16) {
	c.mux.Lock()
	c.topicAlias[alias] = topic
	c.mux.Unlock()
}

func (c *Client) SetConnectProperties(properties *packet.ConnectProperties) {
	c.mux.Lock()
	c.connectProperties = properties
	c.mux.Unlock()
}

func (c *Client) GetID() string {
	return c.ID
}

func (c *Client) MetaString() string {
	var (
		s strings.Builder
	)
	s.WriteString("clientID: ")
	s.WriteString(c.ID)
	s.WriteString(" ")
	s.WriteString("remoteAddr: ")
	s.WriteString(c.conn.RemoteAddr().String())
	return s.String()
}

func (c *Client) GetTopicAlias(u uint16) string {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.topicAlias[u]
}
