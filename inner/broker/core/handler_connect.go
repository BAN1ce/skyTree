package core

import (
	"context"
	"fmt"
	client2 "github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	session2 "github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/BAN1ce/skyTree/pkg/utils"
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

func (c *ConnectHandler) Handle(broker *Broker, client *client2.Client, rawPacket *packets.ControlPacket) error {
	var (
		err error
	)
	// check received connect packet
	if client.IsState(state.ConnectReceived) {
		err = fmt.Errorf("client %s already received connect packet", client.ID)
		return err
	}
	client.SetState(state.ConnectReceived)
	var (
		conAck           = packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
		connectPacket, _ = rawPacket.Content.(*packets.Connect)
	)

	if err = c.handleUsernamePassword(broker, client, connectPacket, conAck); err != nil {
		_ = client.WritePacket(conAck)
		return err
	}

	// handle clean start flag
	if err = c.handleCleanStart(broker, client, *connectPacket, conAck); err != nil {
		_ = client.WritePacket(conAck)
		return err
	}
	broker.CreateClient(client)

	return broker.clientKeepAliveMonitor.SetClientAliveTime(client.UID, utils.NextAliveTime(int64(connectPacket.KeepAlive)))
}

func (c *ConnectHandler) handleUsernamePassword(_ *Broker, _ *client2.Client, packet *packets.Connect, conAck *packets.Connack) error {
	if packet.UsernameFlag == false && packet.Username != "" {
		conAck.ReasonCode = packets.ConnackBadUsernameOrPassword
		return fmt.Errorf("username flag is false, but username is not empty error")
	}
	if (packet.UsernameFlag && packet.Username == "") || (packet.PasswordFlag && len(packet.Password) == 0) {
		conAck.ReasonCode = packets.ConnackBadUsernameOrPassword
		return fmt.Errorf("username or password is empty")
	}

	if (!packet.UsernameFlag && packet.Username != "") || (!packet.PasswordFlag && len(packet.Password) > 0) {
		conAck.ReasonCode = packets.ConnackBadUsernameOrPassword
		return fmt.Errorf("username or password flag error, should empty")
	}
	return nil
}

func (c *ConnectHandler) handleCleanStart(broker *Broker, client *client2.Client, packet packets.Connect, connAck *packets.Connack) error {
	var (
		clientID   = packet.ClientID
		cleanStart = packet.CleanStart
		err        error
		session    session2.Session
		exists     bool
		willCreate bool
	)
	if clientID == "" && !cleanStart {
		connAck.ReasonCode = packets.ConnackInvalidClientID
		return errs.ErrConnackInvalidClientID
	} else if clientID == "" {
		// TODO: generate clientID and confirm protocol
		clientID = uuid.New().String()
	}

	if cleanStart {
		//  release old session
		broker.ReleaseSession(clientID)
		session = broker.sessionManager.NewClientSession(context.TODO(), clientID)
		broker.sessionManager.AddClientSession(context.TODO(), clientID, session)
		logger.Logger.Debug("create new session", zap.String("clientID", clientID))
	} else {
		session, exists = broker.sessionManager.ReadClientSession(context.TODO(), clientID)
		if exists {
			if properties, err := session.GetConnectProperties(); err != nil {
				return err
			} else {
				//  check session expired
				willCreate = properties.IsExpired()
			}
		} else {
			willCreate = true
		}
	}
	if willCreate {
		// create new session
		session = broker.sessionManager.NewClientSession(context.TODO(), clientID)
		broker.sessionManager.AddClientSession(context.TODO(), clientID, session)
	} else {
		connAck.SessionPresent = true
	}

	if err = client.SetComponent(client2.WithSession(session), client2.WithKeepAliveTime(time.Second*time.Duration(packet.KeepAlive))); err != nil {
		connAck.ReasonCode = packets.ConnackServerUnavailable
		return errs.ErrSetClientSession
	}
	client.SetID(clientID)
	return nil
}
