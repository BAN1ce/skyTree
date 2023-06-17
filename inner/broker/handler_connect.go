package broker

import (
	client2 "github.com/BAN1ce/skyTree/inner/broker/client"
	session2 "github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/eclipse/paho.golang/packets"
)

type ConnectHandler struct {
}

func NewConnectHandler() *ConnectHandler {
	return &ConnectHandler{}
}

func (c *ConnectHandler) Handle(broker *Broker, client *client2.Client, packet *packets.ControlPacket) {
	var (
		connectPacket, _   = packet.Content.(*packets.Connect)
		clientID           = connectPacket.ClientID
		session, sessionOk = broker.sessionManager.ReadSession(clientID)
		conAck             = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	)
	// client.SetConnectProperties(packet.NewPahoConnectProperties(connectPacket.Properties))
	// delete old session if clean start is true, and create new session for client
	if connectPacket.CleanStart {
		// clean session
		broker.sessionManager.DeleteSession(clientID)
		session = session2.NewSession()
		client.SetSession(session)
		broker.sessionManager.CreateSession(clientID, session)
	} else if !sessionOk {
		// create new session for client
		session = session2.NewSession()
		broker.sessionManager.CreateSession(clientID, session)
		client.SetSession(session)
		conAck.SessionPresent = true
	} else {
		// use old session
		conAck.SessionPresent = true
		client.SetSession(session)
	}

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

	conAck.ReasonCode = 0x00
	client.SetID(clientID)
	client.SetState(client2.ReceivedConnect)
	broker.CreateClient(client)
	broker.writePacket(client, conAck)
}
