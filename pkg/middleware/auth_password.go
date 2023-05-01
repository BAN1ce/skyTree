package middleware

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/eclipse/paho.golang/packets"
)

type PasswordAuth interface {
	Auth(username, password string) bool
}
type AuthPassword struct {
	auth PasswordAuth
}

func NewAuthPassword(auth PasswordAuth) *AuthPassword {
	return &AuthPassword{auth: auth}
}

func (p *AuthPassword) Handle(client *client.Client, packet *packets.ControlPacket) error {
	con, ok := packet.Content.(*packets.Connect)
	if !ok {
		return nil
	}
	if !p.auth.Auth(con.Username, string(con.Password)) {
		connAck := packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
		connAck.ReasonCode = packets.ConnackBadUsernameOrPassword
		client.WritePacket(connAck)
		return errs.ErrPasswordWrong
	}
	return nil
}
