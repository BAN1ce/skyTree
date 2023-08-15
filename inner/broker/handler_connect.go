package broker

import (
	client2 "github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
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
		// client.proto.Set(pkg.WillFlag, pkg.WillFlagTrue)
		switch connectPacket.WillQOS {
		case 0x00:
			// client.proto.Set(pkg.WillQos, pkg.WillQos0)
		case 0x01:
			// client.proto.Set(pkg.WillQos, pkg.WillQos1)
		case 0x02:
			// client.proto.Set(pkg.WillQos, pkg.WillQos2)
		default:
			conAck.ReasonCode = packets.ConnackProtocolError
			broker.writePacket(client, conAck)
			client.Close()
			return
		}
		if connectPacket.WillRetain {
			// client.proto.Set(pkg.WillRetain, pkg.WillRetainTrue)
		} else {
			// client.proto.Set(pkg.WillRetain, pkg.WillRetainFalse)
		}
		// TODO: handle will message to store client.proto
	} else {
		// client.proto.Set(pkg.WillFlag, pkg.WillFlagFalse)
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
		// clean client.proto
		broker.sessionManager.DeleteSession(clientID)
		session = broker.sessionManager.NewSession(clientID)
		broker.sessionManager.CreateSession(clientID, session)
	} else {
		session, exists = broker.sessionManager.ReadSession(clientID)
		if !exists {
			// create new client.proto for client
			session = broker.sessionManager.NewSession(clientID)
			broker.sessionManager.CreateSession(clientID, session)
		} else {
			// use old client.proto
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
