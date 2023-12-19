package client

import (
	"context"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	topic2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/BAN1ce/skyTree/pkg/utils"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
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
	KeepAlive        time.Duration
}

type Handler interface {
	HandlePacket(*Client, *packets.ControlPacket) error
}

type Client struct {
	ID                   string `json:"id"`
	UID                  string `json:"UID"`
	connectProperties    *session.ConnectProperties
	ctx                  context.Context
	cancel               context.CancelCauseFunc
	mux                  sync.RWMutex
	conn                 net.Conn
	handler              []Handler
	state                state.State
	options              *Options
	packetIDFactory      PacketIDFactory
	publishBucket        *utils.Bucket
	messages             chan pkg.Message
	topicManager         *topic.Manager
	shareTopic           map[string]struct{}
	identifierIDTopic    map[uint16]string
	QoS2                 *QoS2Handler
	topicAliasFromClient map[uint16]string
	subIdentifier        map[string]int
	willFlag             bool
	keepAlive            time.Duration
	aliveTime            time.Time
}

func NewClient(conn net.Conn, option ...Option) *Client {
	var (
		c = &Client{
			UID:                  uuid.NewString(),
			conn:                 conn,
			options:              new(Options),
			identifierIDTopic:    map[uint16]string{},
			topicAliasFromClient: map[uint16]string{},
			shareTopic:           map[string]struct{}{},
		}
	)
	for _, o := range option {
		o(c.options)
	}
	c.packetIDFactory = utils.NewPacketIDFactory()
	c.messages = make(chan pkg.Message, c.options.cfg.WindowSize)
	c.publishBucket = utils.NewBucket(c.options.cfg.WindowSize)
	c.QoS2 = NewQoS2Handler()

	return c
}

func (c *Client) Run(ctx context.Context, handler Handler) {
	c.ctx, c.cancel = context.WithCancelCause(ctx)
	c.topicManager = topic.NewManager(c.ctx)
	c.handler = []Handler{
		handler,
		newInnerHandler(c),
	}
	var (
		controlPacket *packets.ControlPacket
		err           error
	)
	for {
		controlPacket, err = packets.ReadPacket(c.conn)
		if err != nil {
			logger.Logger.Info("read controlPacket error = ", zap.Error(err), zap.String("client", c.MetaString()))
			c.afterClose()
			return
		}
		for _, handler := range c.handler {
			if err := handler.HandlePacket(c, controlPacket); err != nil {
				logger.Logger.Warn("handle packet error", zap.Error(err), zap.String("client", c.MetaString()))
				c.CloseIgnoreError()
				break
			}
		}
	}
}

func (c *Client) HandleSub(subscribe *packets.Subscribe, topics map[string]topic2.Topic) map[string]byte {
	c.mux.Lock()
	defer c.mux.Unlock()
	var (
		failed        = map[string]byte{}
		subIdentifier int
	)
	if subscribe != nil {
		// store topic's subIdentifier
		if tmp := subscribe.Properties.SubscriptionIdentifier; tmp != nil {
			subIdentifier = *tmp
			for _, t := range subscribe.Subscriptions {
				c.subIdentifier[t.Topic] = subIdentifier
			}
		}
	}
	// create topic instance

	for topicName, t := range topics {
		c.topicManager.AddTopic(topicName, t)
	}
	return failed
}

func (c *Client) AddTopics(topic map[string]topic2.Topic) {
	c.mux.Lock()
	defer c.mux.Unlock()
	for topicName, t := range topic {
		c.topicManager.AddTopic(topicName, t)
	}
}

func (c *Client) readUnfinishedMessage(topic string) []*packet.Message {
	if c.options.session == nil {
		logger.Logger.Warn("session is nil", zap.String("client", c.MetaString()))
		return nil
	}
	return c.options.session.ReadTopicUnFinishedMessage(topic)
}

func (c *Client) HandleUnSub(topicName string) {
	// TODO: should finish all cache message before delete topic.
	c.mux.Lock()
	defer c.mux.Unlock()
	c.options.session.DeleteSubTopic(topicName)
	c.topicManager.DeleteTopic(topicName)
}

func (c *Client) HandlePubAck(pubAck *packets.Puback) {
	topicName := c.identifierIDTopic[pubAck.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubAck packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubAck.PacketID))
		return
	}
	c.publishBucket.PutToken()
	c.topicManager.HandlePublishAck(topicName, pubAck)
}

func (c *Client) HandlePubRec(pubRec *packets.Pubrec) {
	topicName := c.identifierIDTopic[pubRec.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubRec packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRec.PacketID))
		return
	}
	c.topicManager.HandlePublishRec(topicName, pubRec)
}

func (c *Client) HandlePubComp(pubRel *packets.Pubcomp) {
	topicName := c.identifierIDTopic[pubRel.PacketID]
	if len(topicName) == 0 {
		logger.Logger.Warn("pubComp packetID not found store", zap.String("client", c.MetaString()), zap.Uint16("packetID", pubRel.PacketID))
		return
	}
	c.publishBucket.PutToken()
	c.topicManager.HandelPublishComp(topicName, pubRel)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) CloseIgnoreError() {
	if err := c.conn.Close(); err != nil {
		logger.Logger.Warn("close conn error", zap.Error(err), zap.String("client", c.MetaString()))
	}
}

func (c *Client) afterClose() {
	logger.Logger.Info("client close", zap.String("clientID", c.ID))
	defer func() {
		c.options.notifyClose.NotifyClientClose(c)
	}()

	if err := c.conn.Close(); err != nil {
		logger.Logger.Info("close conn error", zap.Error(err), zap.String("client", c.MetaString()))
	}

	//  close normal topicManager
	if err := c.topicManager.Close(); err != nil {
		logger.Logger.Warn("close topicManager error", zap.Error(err), zap.String("client", c.MetaString()))
	}

	//  close share topicManager

	if c.options.session == nil {
		logger.Logger.Warn("session is nil", zap.String("client", c.MetaString()))
		return
	}

	// store unfinished message to session for qos1 and qos2
	c.storeUnfinishedMessage()

	//  handle will message
	willMessage, ok, err := c.options.session.GetWillMessage()
	if err != nil {
		logger.Logger.Error("get will message error", zap.Error(err), zap.String("client", c.MetaString()))
		return
	}
	if !ok {
		return
	}

	var (
		willDelayInterval = willMessage.Property.GetDelayInterval()
	)

	if willDelayInterval == 0 {
		c.options.notifyClose.NotifyWillMessage(willMessage)
		return
	}

	// create a delay task to publish will message
	if err := facade.GetWillDelay().Create(&retry.Task{
		MaxTimes:     1,
		MaxTime:      0,
		IntervalTime: willDelayInterval,
		Key:          willMessage.DelayTaskID,
		Data:         willMessage,
		Job: func(task *retry.Task) {
			if m, ok := task.Data.(*session.WillMessage); ok {
				logger.Logger.Debug("will message delay task", zap.String("client", c.MetaString()), zap.String("delayTaskID", task.Key))
				c.options.notifyClose.NotifyWillMessage(m)
			} else {
				logger.Logger.Error("will message type error", zap.String("client", c.MetaString()))
			}
		},
		TimeoutJob: nil,
	}); err != nil {
		logger.Logger.Error("create will delay task error", zap.Error(err), zap.String("client", c.MetaString()))
		return
	}

}

func (c *Client) storeUnfinishedMessage() {
	var message = c.topicManager.GetUnfinishedMessage()
	if c.options.session == nil {
		return
	}
	for topicName, message := range message {
		logger.Logger.Debug("store unfinished message", zap.String("client", c.MetaString()), zap.String("topicName", topicName), zap.Int("message", len(message)))
		c.options.session.CreateTopicUnFinishedMessage(topicName, message)
	}
}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}

func (c *Client) WritePacket(packet packets.Packet) error {
	if packet == nil {
		return nil
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	logger.Logger.Debug("write packet", zap.String("client", c.MetaString()), zap.Any("packet", packet))
	return c.writePacket(packet)
}

func (c *Client) writePacket(packet packets.Packet) error {
	var (
		buf              = pool.ByteBufferPool.Get()
		prepareWriteSize int64
		err              error
		topicName        string
	)
	defer pool.ByteBufferPool.Put(buf)
	// publishAck, subscribeAck, unsubscribeAck should use the same packetID as the original packet

	switch p := packet.(type) {

	case *packets.Connack:
		// do plugin
		c.options.plugin.DoSendConnAck(c.ID, p)

	case *packets.Suback:
		// do plugin
		c.options.plugin.DoSendSubAck(c.ID, p)

	case *packets.Unsuback:
		// do plugin
		c.options.plugin.DoSendUnsubAck(c.ID, p)

	case *packets.Publish:
		if p.QoS != broker.QoS0 {
			// request publish bucket
			select {
			case <-c.ctx.Done():
				return c.ctx.Err()
			case <-c.publishBucket.GetToken():
			}
		}
		// do plugin
		c.options.plugin.DoSendPublish(c.ID, p)
		topicName = p.Topic
		// generate new packetID and store
		p.PacketID = c.packetIDFactory.Generate()
		c.identifierIDTopic[p.PacketID] = p.Topic
		logger.Logger.Debug("publish to client", zap.Uint16("packetID", p.PacketID), zap.String("client", c.MetaString()), zap.String("store", p.Topic))

		// append subscriptionIdentifier
		if subIdentifier, ok := c.subIdentifier[p.Topic]; ok {
			p.Properties.SubscriptionIdentifier = &subIdentifier
		}

	case *packets.Puback:
		// do plugin
		c.options.plugin.DoSendPubAck(c.ID, p)

	case *packets.Pubrec:
		// do plugin
		c.options.plugin.DoSendPubRec(c.ID, p)

	case *packets.Pubrel:
		// do plugin
		c.options.plugin.DoSendPubRel(c.ID, p)

	case *packets.Pubcomp:
		// do plugin
		c.options.plugin.DoSendPubComp(c.ID, p)

	case *packets.Pingresp:
		// do plugin
		c.options.plugin.DoSendPingResp(c.ID, p)

	}

	if prepareWriteSize, err = packet.WriteTo(buf); err != nil {
		logger.Logger.Info("write packet error", zap.Error(err), zap.String("client", c.MetaString()))
		// TODO: check maximum packet size should close client ?
	}

	// check maximum packet size
	if c.connectProperties != nil && c.connectProperties.MaximumPacketSize != nil && *c.connectProperties.MaximumPacketSize != 0 {
		if buf.Len() > int(*c.connectProperties.MaximumPacketSize) && *c.connectProperties.MaximumPacketSize != 0 {
			return errs.ErrPacketOversize
		}
	}
	if _, err = c.conn.Write(buf.Bytes()); err != nil {
		logger.Logger.Info("write packet error", zap.Error(err), zap.String("client", c.MetaString()), zap.Int64("prepareWriteSize", prepareWriteSize), zap.String("topicName", topicName))
		// TODO: check maximum packet size should close client ?
		if err := c.conn.Close(); err != nil {
			logger.Logger.Warn("close conn error", zap.Error(err), zap.String("client", c.MetaString()))
		}
		return err
	}
	return nil
}

func (c *Client) SetWithOption(ops ...Option) error {
	c.mux.Lock()
	for _, o := range ops {
		o(c.options)
	}

	c.mux.Unlock()
	return nil
}

type SessionTopicData struct {
	SubOption  *proto.SubOption
	UnFinished []*packet.Message
}

func (c *Client) getSession() session.Session {
	return c.options.session
}

func (c *Client) setWill(message *session.WillMessage) error {
	c.willFlag = true
	if oldWillMessage, ok, _ := c.options.session.GetWillMessage(); ok {
		facade.GetWillDelay().Delete(oldWillMessage.DelayTaskID)
	}
	return c.options.session.SetWillMessage(message)
}
func (c *Client) DeleteWill() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.options.session.DeleteWillMessage()
}

func (c *Client) SetClientTopicAlias(topic string, alias uint16) {
	c.mux.Lock()
	c.topicAliasFromClient[alias] = topic
	c.mux.Unlock()
}

func (c *Client) SetConnectProperties(properties *session.ConnectProperties) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.connectProperties = properties
	return c.options.session.SetConnectProperties(properties)
}
func (c *Client) GetConnectProperties() session.ConnectProperties {
	c.mux.RLock()
	defer c.mux.RUnlock()
	if c.connectProperties == nil {
		return session.ConnectProperties{}
	}
	return *c.connectProperties
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

func (c *Client) GetClientTopicAlias(u uint16) string {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.topicAliasFromClient[u]
}

func (c *Client) Publish(topic string, message *packet.Message) error {
	return c.topicManager.Publish(topic, message)
}

func (c *Client) RefreshAliveTime() {
	c.aliveTime = time.Now()
}

func (c *Client) IsPingTimeout() bool {
	return time.Since(c.aliveTime) > c.keepAlive
}
