package client

import (
	"container/list"
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/BAN1ce/skyTree/pkg/util"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
	"net"
	"strconv"
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
	ListenClientClose(*Client)
	HandlePacket(*Client, *packets.ControlPacket)
}

type Client struct {
	ctx               context.Context
	mux               sync.RWMutex
	conn              net.Conn
	ID                string
	handler           Handler
	closeOnce         sync.Once
	state             state.State
	options           *Options
	session           pkg.Session
	waitAck           *list.List
	packetIDFactory   PacketIDFactory
	publishBucket     *util.Bucket
	messages          chan pkg.Message
	connectProperties paho.ConnectProperties
}

func NewClient(conn net.Conn, option ...Option) *Client {
	var (
		c = &Client{
			conn:    conn,
			options: new(Options),
			waitAck: list.New(),
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
		packet *packets.ControlPacket
		err    error
	)
	go func() {
		for {
			packet, err = packets.ReadPacket(c.conn)
			if err != nil {
				logger.Logger.Error("read packet error = ", err.Error())
				c.Close()
				return
			}
			handler.HandlePacket(c, packet)
		}
	}()
}

func (c *Client) RunSession() {
	go func() {
		c.readMessages()
	}()
}

func (c *Client) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.messages:
			// TODO: record message's id to session
			// publish message to client
			c.pushToWindow(msg)
		default:
			c.readStoreToFillChannel()
			if len(c.messages) > 0 {
				continue
			}
			// waiting for store event
			<-c.storeReady().Done()
		}
	}
}

func (c *Client) storeReady() context.Context {
	var (
		ctx, cancel = context.WithCancel(c.ctx)
	)
	c.session.OnceListenPublishEvent(c.ID, func(topic, id string) {
		c.mux.Lock()
		defer c.mux.Unlock()
		if c.readMessageStoreByID(topic, id) > 0 {
			cancel()
			return
		}
		logger.Logger.Warn("readMessageStoreByID: no message", "topic = ", topic, "id = ", id)
		cancel()
	})
	return ctx
}

// readStoreToFillChannel read message from store util fill the messages channel or all topics done
func (c *Client) readStoreToFillChannel() {
	var (
		i int
	)
	// TODO: prevent some topics hunger
	for topic, id := range c.session.GetTopicsMessageID() {
		i += c.readMessageStoreByID(topic, id)
		if i >= c.options.cfg.WindowSize {
			return
		}
	}
}

func (c *Client) readMessageStoreByID(topic, id string) int {
	var (
		i           int
		ctx, cancel = context.WithTimeout(c.ctx, c.options.cfg.ReadStoreTimeout)
	)
	defer cancel()
	if message := c.options.Store.ReadTopicMessageByID(ctx, topic, id, c.options.cfg.WindowSize); len(message) > 0 {
		for _, msg := range message {
			select {
			case c.messages <- msg:
				i++
			default:
			}
		}
	}
	return i
}

func (c *Client) pushToWindow(msg pkg.Message) {
	select {
	case <-c.ctx.Done():
		return
	case <-c.publishBucket.GetToken():
		pub := packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
		pub.Payload = msg.GetPayload()
		pub.Topic = msg.GetTopic()
		pub.PacketID = c.packetIDFactory.Generate()
		c.writePacket(pub)
		c.waitAck.PushBack(newWaitAckPacket(pub, msg.GetID()))
	}
}

func (c *Client) HandlePubAck(pubAck packets.Puback) {
	for e := c.waitAck.Front(); e != nil; e = e.Next() {
		if e.Value.(*waitAckPacket).packet.PacketID == pubAck.PacketID {
			e.Value.(*waitAckPacket).ack = true
			// Think about it, if the ack is not in order, it will cause the ack to be blocked, should release a bucket here ?
			break
		}
	}
	for e := c.waitAck.Front(); e != nil; e = e.Next() {
		waitAckPacket := e.Value.(*waitAckPacket)
		if waitAckPacket.ack {
			break
		}
		c.waitAck.Remove(e)
		// Fixme: bucket leak
		c.session.CreateTopicMessageID(waitAckPacket.packet.Topic, waitAckPacket.messageID)
		c.publishBucket.PutToken()
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		logger.Logger.Info("client close = ", c.ID)
		if err := c.conn.Close(); err != nil {
			logger.Logger.Error("close client error = ", err.Error())
		}
		// TODO: check will message
		c.handler.ListenClientClose(c)
	})

}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}

func (c *Client) SetState(state uint64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.state.SetState(state)
}

func (c *Client) IsState(state uint64) bool {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.state.IsState(state)
}

func (c *Client) RemState(state uint64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.state.RemState(state)
}

func (c *Client) WritePacket(packet packets.Packet) {
	if _, err := packet.WriteTo(c); err != nil {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
}

func (c *Client) writePacket(packet packets.Packet) {
	if _, err := packet.WriteTo(c); err != nil {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
}

func (c *Client) SetSession(session pkg.Session) {
	c.mux.Lock()
	c.session = session
	c.mux.Unlock()
}

func (c *Client) GetSession() pkg.Session {
	return c.session
}

func (c *Client) DestroySession() {
	c.mux.Lock()
	c.session.Destroy()
	c.mux.Unlock()
}

func (c *Client) SetLastAliveTime(time int64) {
	c.mux.Lock()
	c.session.Set(pkg.LastAliveTime, strconv.FormatInt(time, 10))
	c.mux.Unlock()
}

func (c *Client) SetWillMessage(message string) {
	c.mux.Lock()
	c.session.Set(pkg.WillMessage, message)
	c.mux.Unlock()
}
