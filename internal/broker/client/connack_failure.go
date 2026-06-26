package client

import (
	"errors"

	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

// failConnect 写出失败 CONNACK，并按 spec §3.2.2.3.9/3.2.2.3.10
// 携带 ReasonString，便于排障。
//
// 调用方仍需要返回原 error，failConnect 不负责 panic / close 连接。
// applyProblemInfoPreference 在 client.write 链路上被自动应用，
// 但 spec §3.1.2.11.7 允许 CONNACK 即便 RequestProblemInfo=0 也保留 ReasonString，
// 所以这里直接填即可。
func (c *Client) failConnect(reasonCode byte, reasonString string) error {
	cp := packets.NewControlPacket(packets.CONNACK)
	conAck := &packets.ConnAck{ReasonCode: reasonCode}
	if reasonString != "" {
		conAck.Properties = &packets.ConnAckProperties{ReasonString: reasonString}
	}
	cp.Content = conAck
	return c.write(&clientcap.WritePacket{Packet: cp})
}

func (c *Client) failConnectForReadPacketError(err error) error {
	if c == nil || err == nil {
		return nil
	}
	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) || wireErr.Packet != packets.CONNECT {
		return nil
	}
	return c.failConnect(connAckReasonFromWireError(wireErr), wireErr.Error())
}

func connAckReasonFromWireError(err *wire.WireError) byte {
	if err == nil {
		return packets.ConnAckUnspecifiedError
	}
	switch err.Kind {
	case wire.ErrMalformedPacket:
		return packets.ConnAckMalformedPacket
	case wire.ErrProtocolError:
		return packets.ConnAckProtocolError
	case wire.ErrPacketTooLarge:
		return packets.ConnAckPacketTooLarge
	case wire.ErrUnsupportedProtocolVersion:
		return packets.ConnAckUnsupportedProtocolVersion
	case wire.ErrImplementation:
		return packets.ConnAckImplementationSpecificError
	default:
		return packets.ConnAckUnspecifiedError
	}
}
