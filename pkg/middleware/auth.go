package middleware

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/eclipse/paho.golang/packets"
)

type AuthHandler interface {
	Auth(method string, data []byte) error
}
type Auth struct {
	auth AuthHandler
}

func NewAuth(handler AuthHandler) *Auth {
	return &Auth{auth: handler}
}

func (a *Auth) Handle(client *client.Client, packet *packets.ControlPacket) error {
	var (
		authPacket = packet.Content.(*packets.Auth)
		rsp        = packets.NewControlPacket(packets.AUTH).Content.(*packets.Auth)
	)
	if authPacket.Properties == nil {
		return errs.ErrAuthHandlerNotSet
	}
	if err := a.auth.Auth(authPacket.Properties.AuthMethod, authPacket.Properties.AuthData); err != nil {
		return err
	}
	rsp.ReasonCode = 0x00
	client.WritePacket(rsp)
	return nil
}
