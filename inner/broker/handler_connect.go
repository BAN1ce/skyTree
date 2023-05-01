package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	session2 "github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/eclipse/paho.golang/packets"
)

type ConnectHandler struct {
}

func NewConnectHandler() *ConnectHandler {
	return &ConnectHandler{}
}
func (c *ConnectHandler) Handle(broker *Broker, client *client.Client, packet *packets.ControlPacket) {
	var (
		connectPacket, _   = packet.Content.(*packets.Connect)
		clientID           = connectPacket.ClientID
		session, sessionOk = broker.sessionManager.ReadSession(clientID)
		conAck             = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	)

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
		// TODO: handle will message to store session
	}

	conAck.ReasonCode = 0x00
	client.SetID(clientID)
	broker.clientManager.CreateClient(client)
	broker.writePacket(client, conAck)
}
