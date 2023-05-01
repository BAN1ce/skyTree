package middleware

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/eclipse/paho.golang/packets"
)

type ProtocolVersion struct {
}

func (p *ProtocolVersion) Handle(client *client.Client, packet *packets.ControlPacket) error {
	con, ok := packet.Content.(*packets.Connect)
	if !ok {
		logger.Logger.Error("packet content is not connect")
		return nil
	}
	if con.ProtocolVersion != 0x05 || con.ProtocolName != `MQTT` {
		connAck := packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
		connAck.ReasonCode = packets.ConnackProtocolError
		client.WritePacket(connAck)
		return errs.ErrProtocolNotSupport
	}
	return nil
}
