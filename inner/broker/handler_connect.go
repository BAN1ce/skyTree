package broker

import (
	client2 "github.com/BAN1ce/skyTree/inner/broker/client"
	session2 "github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/errs"
	packet2 "github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	} else {
		client.SetState(state.ConnectReceived)
	}
	var (
		err              error
		connectPacket, _ = packet.Content.(*packets.Connect)
		conAck           = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	)
	// default ack code is success
	conAck.ReasonCode = packets.ConnackSuccess
	// parse connect properties and check
	if err = c.handleConnectProperties(broker, client, connectPacket, conAck); err != nil {
		broker.writePacket(client, conAck)
		client.Close()
		return
	}
	// handle clean start flag
	if err = c.handleCleanStart(broker, client, *connectPacket, conAck); err != nil {
		logger.Logger.Warn("handle clean start error: ", zap.Error(err), zap.String("client", client.MetaString()))
		broker.writePacket(client, conAck)
		client.Close()
		return
	}
	// TODO: handle will message
	if connectPacket.WillFlag {
		// session.Set(pkg.WillFlag, pkg.WillFlagTrue)
		switch connectPacket.WillQOS {
		case 0x00:
			// session.Set(pkg.WillQos, pkg.WillQos0)
		case 0x01:
			// session.Set(pkg.WillQos, pkg.WillQos1)
		case 0x02:
			// session.Set(pkg.WillQos, pkg.WillQos2)
		default:
			conAck.ReasonCode = packets.ConnackProtocolError
			broker.writePacket(client, conAck)
			client.Close()
			return
		}
		if connectPacket.WillRetain {
			// session.Set(pkg.WillRetain, pkg.WillRetainTrue)
		} else {
			// session.Set(pkg.WillRetain, pkg.WillRetainFalse)
		}
		// TODO: handle will message to store session
	} else {
		// session.Set(pkg.WillFlag, pkg.WillFlagFalse)
	}

	// TODO: connect ack properties implementation
	broker.CreateClient(client)
	broker.writePacket(client, conAck)
}

func (c *ConnectHandler) handleConnectProperties(_ *Broker, client *client2.Client, packet *packets.Connect, conack *packets.Connack) error {
	var connectProperties, err = packet2.PropertyToConnectProperties(packet.Properties)
	if err != nil {
		conack.ReasonCode = packets.ConnackProtocolError
		return err
	}
	client.SetConnectProperties(connectProperties)
	return nil
}

// handleCleanStart handle clean start flag, if clean start is true, delete session from session manager and create new session for client.
// if clean start is false, read session from session manager, if session not exists, create new session for client, else use old session.
// clean start is false, clientID must not be empty.
func (c *ConnectHandler) handleCleanStart(broker *Broker, client *client2.Client, packet packets.Connect, connack *packets.Connack) error {
	var (
		clientID   = packet.ClientID
		cleanStart = packet.CleanStart
		err        error
		session    pkg.Session
		exists     bool
	)
	if clientID == "" && !cleanStart {
		connack.ReasonCode = packets.ConnackInvalidClientID
		return errs.ErrConnackInvalidClientID
	} else if clientID == "" {
		clientID = uuid.New().String()
	}
	if cleanStart {
		// clean session
		broker.sessionManager.DeleteSession(clientID)
		session = session2.NewSession()
		broker.sessionManager.CreateSession(clientID, session)
	} else {
		session, exists = broker.sessionManager.ReadSession(clientID)
		if !exists {
			// create new session for client
			session = session2.NewSession()
			broker.sessionManager.CreateSession(clientID, session)
		} else {
			// use old session
			connack.SessionPresent = true
		}
	}
	if err = client.SetSession(session); err != nil {
		connack.ReasonCode = packets.ConnackServerUnavailable
		return errs.ErrSetClientSession
	}
	client.SetID(clientID)
	return nil
}

func (c *ConnectHandler) handleWill(broker *Broker, client *client2.Client, packet packets.Connect) (byte, error) {
	if !packet.WillFlag {
		return packets.ConnackSuccess, nil
	}
	// var (
	// 	willTopic      = packet.WillTopic
	// 	willMessage    = packet.WillMessage
	// 	willQos        = packet.WillQOS
	// 	willRetain     = packet.WillRetain
	// 	willProperties = packet2.PropertyToWillProperties(packet.Properties)
	// )
	return packets.ConnackSuccess, nil
}
