package broker

import (
	client2 "github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/errs"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"time"
)

type ConnectHandler struct {
}

func NewConnectHandler() *ConnectHandler {
	return &ConnectHandler{}
}

func (c *ConnectHandler) Handle(broker *Broker, client *client2.Client, packet *packets.ControlPacket) {
	if client.IsState(state.ConnectReceived) {
		// client already received connect packet
		client.Close()
		return
	}
	client.SetState(state.ConnectReceived)
	var (
		err              error
		connectPacket, _ = packet.Content.(*packets.Connect)
		conAck           = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	)
	conAck.ReasonCode = packets.ConnackServerUnavailable

	// handle clean start flag
	if err = c.handleCleanStart(broker, client, *connectPacket, conAck); err != nil {
		logger.Logger.Warn("handle clean start error: ", zap.Error(err), zap.String("client", client.MetaString()))
		broker.writePacket(client, conAck)
		client.Close()
		return
	}

	// parse connect properties and check
	if err = c.handleConnectProperties(broker, client, connectPacket, conAck); err != nil {
		logger.Logger.Warn("handle connect properties error: ", zap.Error(err), zap.String("client", client.MetaString()))
		broker.writePacket(client, conAck)
		client.Close()
		return
	}

	// handle will message
	if err = c.handleWillMessage(broker, connectPacket, client.GetSession()); err != nil {
		logger.Logger.Warn("handle will message error: ", zap.Error(err), zap.String("client", client.MetaString()))
		broker.writePacket(client, conAck)
		client.Close()
		return
	}

	conAck.ReasonCode = packets.ConnackSuccess
	broker.CreateClient(client)
	broker.writePacket(client, conAck)
}

func (c *ConnectHandler) handleConnectProperties(_ *Broker, client *client2.Client, packet *packets.Connect, conack *packets.Connack) error {
	var connectProperties, err = packet2.PropertyToConnectProperties(packet.Properties)
	if err != nil {
		conack.ReasonCode = packets.ConnackProtocolError
		return err
	}
	client.SetConnectProperties(&broker2.SessionConnectProperties{
		ExpiryInterval: connectProperties.SessionExpiryInterval,
	})
	return nil
}

// handleCleanStart handle clean start flag, if clean start is true, delete client.proto from client.proto manager and create new client.proto for client.
// if clean start is false, read client.proto from client.proto manager, if client.proto not exists, create new client.proto for client, else use old client.proto.
// clean start is false, clientID must not be empty.
func (c *ConnectHandler) handleCleanStart(broker *Broker, client *client2.Client, packet packets.Connect, connack *packets.Connack) error {
	var (
		clientID   = packet.ClientID
		cleanStart = packet.CleanStart
		err        error
		session    broker2.Session
		exists     bool
	)
	if clientID == "" && !cleanStart {
		connack.ReasonCode = packets.ConnackInvalidClientID
		return errs.ErrConnackInvalidClientID
	} else if clientID == "" {
		clientID = uuid.New().String()
	}
	if cleanStart {
		broker.ReleaseSession(clientID)
		session = broker.sessionManager.NewSession(clientID)
		broker.sessionManager.CreateSession(clientID, session)
	} else {
		session, exists = broker.sessionManager.ReadSession(clientID)
		if !exists {
			session = broker.sessionManager.NewSession(clientID)
			broker.sessionManager.CreateSession(clientID, session)
		} else {
			broker.ReleaseWillMessage(session)
			// use old client.proto
			connack.SessionPresent = true
		}
	}
	if err = client.SetMeta(session, time.Second*time.Duration(packet.KeepAlive)); err != nil {
		connack.ReasonCode = packets.ConnackServerUnavailable
		return errs.ErrSetClientSession
	}
	client.SetID(clientID)
	return nil
}

// handleWillMessage handle will message, if willing flag is true, store will message to store and set will message id to broker state.
func (c *ConnectHandler) handleWillMessage(broker *Broker, connect *packets.Connect, session broker2.Session) error {
	if !connect.WillFlag {
		return nil
	}
	var (
		publishPacket = pool.PublishPool.Get()
	)
	publishPacket.Topic = connect.WillTopic
	publishPacket.Payload = connect.WillMessage
	publishPacket.QoS = connect.WillQOS
	publishPacket.Retain = connect.WillRetain

	// TODO: fill publish message properties and other fields
	messageID, err := broker.store.StorePublishPacket(map[string]int32{
		connect.WillTopic: int32(connect.WillQOS),
	}, publishPacket)
	if err != nil {
		return err
	}

	if err := broker.state.CreateTopicWillMessageID(connect.WillTopic, messageID, connect.WillRetain); err != nil {
		return err
	}
	return session.SetWillMessage(broker2.ConnectPacketToWillMessage(connect, messageID))
}
