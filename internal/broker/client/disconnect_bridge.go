package client

import (
	"time"

	disconnect "github.com/BAN1ce/skyTree/internal/broker/client/internal/disconnect"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

type DisconnectOption = disconnect.Option

func WithServerReference(ref string) DisconnectOption {
	return disconnect.WithServerReference(ref)
}

func WithUserProperty(key, value string) DisconnectOption {
	return disconnect.WithUserProperty(key, value)
}

func newServerDisconnect(code byte, reason string, opts ...DisconnectOption) *packets.ControlPacket {
	return disconnect.NewServerDisconnect(code, reason, opts...)
}

func newServerDisconnectMinimal(code byte) *packets.ControlPacket {
	return disconnect.NewServerDisconnectMinimal(code)
}

func disconnectForProtocolError(reason string) *packets.ControlPacket {
	return disconnect.ForProtocolError(reason)
}

func disconnectForSessionTakenOver() *packets.ControlPacket {
	return disconnect.ForSessionTakenOver()
}

// DisconnectForKeepAliveTimeout 用于 KeepAlive 超时主动下发的 DISCONNECT。
// 暴露给 broker 层调用。
func DisconnectForKeepAliveTimeout(idle, configured time.Duration) *packets.ControlPacket {
	return disconnect.ForKeepAliveTimeout(idle, configured)
}

// DisconnectForServerShuttingDown 用于服务端优雅关停时下发的 DISCONNECT。
// 暴露给 broker 层调用。
func DisconnectForServerShuttingDown() *packets.ControlPacket {
	return disconnect.ForServerShuttingDown()
}

// DisconnectForQuotaExceeded is used by broker-level callbacks when a server-side
// quota or retry limit is exceeded after the connection is established.
func DisconnectForQuotaExceeded(reason string) *packets.ControlPacket {
	return disconnect.ForQuotaExceeded(reason)
}

func disconnectForQuotaExceeded(reason string) *packets.ControlPacket {
	return disconnect.ForQuotaExceeded(reason)
}

func disconnectForAdministrativeAction(reason string) *packets.ControlPacket {
	return disconnect.ForAdministrativeAction(reason)
}

func disconnectForMessageRateTooHigh(reason string) *packets.ControlPacket {
	return disconnect.ForMessageRateTooHigh(reason)
}

func disconnectForPacketTooLarge(packetSize, maxSize int) *packets.ControlPacket {
	return disconnect.ForPacketTooLarge(packetSize, maxSize)
}

func minimalDisconnectForPacketTooLarge() *packets.ControlPacket {
	return disconnect.MinimalForPacketTooLarge()
}

func disconnectForReceiveMaximumExceeded(inflight, maximum int) *packets.ControlPacket {
	return disconnect.ForReceiveMaximumExceeded(inflight, maximum)
}

func disconnectForTopicAliasError(err error) *packets.ControlPacket {
	return disconnect.ForTopicAliasError(err)
}
