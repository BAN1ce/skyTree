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
	"net"
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
	ID                string
	connectProperties *packet.ConnectProperties
	ctx               context.Context
	mux               sync.RWMutex
	conn              net.Conn
	handler           Handler
	closeOnce         sync.Once
	state             state.State
	options           *Options
	packetIDFactory   PacketIDFactory
	publishBucket     *util.Bucket
	messages          chan pkg.Message
	topics            *topic.Topics
}

func NewClient(conn net.Conn, option ...Option) *Client {
	var (
		c = &Client{
			conn:    conn,
			options: new(Options),
		}
	)
	for _, o := range option {
		o(c.options)
	}
	c.packetIDFactory = util.NewPacketIDFactory()
	c.messages = make(chan pkg.Message, c.options.cfg.WindowSize)
	c.publishBucket = util.NewBucket(c.options.cfg.WindowSize)
	return c
}

func (c *Client) Write(data []byte) (int, error) {
	return c.conn.Write(data)
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
			logger.Logger.Error("read controlPacket error = ", err.Error())
			c.Close()
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
		if err := c.topics.CreateTopic(t, v.QoS); err != nil {
			failed[t] = 0x80
		} else {
			failed[t] = v.QoS
		}
	}
	return failed
}

func (c *Client) HandleUnSub(topicName string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.options.session.DeleteSubTopics(topicName)
	c.topics.DeleteTopic(topicName)
}

func (c *Client) HandlePubAck(pubAck packets.Puback) {

}

func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		logger.Logger.Info("client close = ", c.ID)
		if err := c.conn.Close(); err != nil {
			logger.Logger.Info("close client error = ", err.Error())
		}
		if err := c.topics.Close(); err != nil {
			logger.Logger.Warn("close topics error = ", err.Error())
		}
		c.options.notifyClose.NotifyClientClose(c)
		// TODO: check will message
	})
	return nil
}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}

func (c *Client) WritePacket(packet packets.Packet) {
	c.writePacket(packet)
}

func (c *Client) writePacket(packet packets.Packet) {
	var (
		buf              = pool.ByteBufferPool.Get()
		prepareWriteSize int64
		err              error
	)
	// publishAck, subscribeAck, unsubscribeAck should use the same packetID as the original packet
	switch packet.(type) {
	case *packets.Publish:
		packet.(*packets.Publish).PacketID = c.packetIDFactory.Generate()
	}
	defer pool.ByteBufferPool.Put(buf)
	if prepareWriteSize, err = packet.WriteTo(buf); err != nil {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
	// TODO: check maximum packet size
	if prepareWriteSize < 0 {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
	if _, err = c.conn.Write(buf.Bytes()); err != nil {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
}

func (c *Client) SetSession(session pkg.Session) error {
	var (
		err error
	)
	c.mux.Lock()
	c.options.session = session
	c.topics, err = topic.NewTopicWithSession(c.ctx, session,
		topic.WithStore(c.options.Store),
		topic.WithWriter(c),
	)
	c.mux.Unlock()
	return err
}

func (c *Client) SetWill() {

}

func (c *Client) SetConnectProperties(properties *packet.ConnectProperties) {
	c.mux.Lock()
	c.connectProperties = properties
	c.mux.Unlock()
}
