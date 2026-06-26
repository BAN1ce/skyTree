package disconnect

import (
	"fmt"
	"time"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ForProtocolError creates a DISCONNECT packet for MQTT protocol errors.
// Reason code: Protocol Error (0x82).
func ForProtocolError(reason string) *packets.ControlPacket {
	if reason == "" {
		return NewServerDisconnect(packets.DisconnectProtocolError, "protocol error")
	}
	return NewServerDisconnect(
		packets.DisconnectProtocolError,
		fmt.Sprintf("protocol error: %s", reason),
	)
}

func ForSessionTakenOver() *packets.ControlPacket {
	return NewServerDisconnect(packets.DisconnectSessionTakenOver, "session taken over")
}

// ForKeepAliveTimeout 用于 KeepAlive 超时主动下发的 DISCONNECT。
func ForKeepAliveTimeout(idle, configured time.Duration) *packets.ControlPacket {
	return NewServerDisconnect(
		packets.DisconnectKeepAliveTimeout,
		fmt.Sprintf("keepalive timeout: idle %s exceeds %s (1.5x)", idle, configured),
	)
}

// ForServerShuttingDown 用于服务端优雅关停时下发的 DISCONNECT。
func ForServerShuttingDown() *packets.ControlPacket {
	return NewServerDisconnect(
		packets.DisconnectServerShuttingDown,
		"server shutting down",
	)
}

// ForQuotaExceeded 用于服务端因 PacketID 耗尽 / inflight 超时等场景的 DISCONNECT。
func ForQuotaExceeded(reason string) *packets.ControlPacket {
	if reason == "" {
		return NewServerDisconnect(packets.DisconnectQuotaExceeded, "quota exceeded")
	}
	return NewServerDisconnect(
		packets.DisconnectQuotaExceeded,
		fmt.Sprintf("quota exceeded: %s", reason),
	)
}

// ForAdministrativeAction 用于管理员强制下线场景。
func ForAdministrativeAction(reason string) *packets.ControlPacket {
	if reason == "" {
		return NewServerDisconnect(packets.DisconnectAdministrativeAction, "administrative action")
	}
	return NewServerDisconnect(
		packets.DisconnectAdministrativeAction,
		fmt.Sprintf("administrative action: %s", reason),
	)
}

// ForMessageRateTooHigh 用于消息速率超限的场景。
func ForMessageRateTooHigh(reason string) *packets.ControlPacket {
	if reason == "" {
		return NewServerDisconnect(packets.DisconnectMessageRateTooHigh, "message rate too high")
	}
	return NewServerDisconnect(
		packets.DisconnectMessageRateTooHigh,
		fmt.Sprintf("message rate too high: %s", reason),
	)
}
