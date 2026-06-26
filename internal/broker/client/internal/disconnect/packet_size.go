package disconnect

import (
	"fmt"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ForPacketTooLarge creates a DISCONNECT packet for Maximum Packet Size violations.
func ForPacketTooLarge(packetSize, maxSize int) *packets.ControlPacket {
	return NewServerDisconnect(
		packets.DisconnectPacketTooLarge,
		fmt.Sprintf("packet size %d exceeds maximum %d", packetSize, maxSize),
	)
}

func MinimalForPacketTooLarge() *packets.ControlPacket {
	return NewServerDisconnectMinimal(packets.DisconnectPacketTooLarge)
}
