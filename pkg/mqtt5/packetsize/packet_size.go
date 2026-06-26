package packetsize

import (
	"fmt"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

// CheckPacketSize checks if packet size exceeds a maximum.
func CheckPacketSize(packet *packets.ControlPacket, maxSize int) error {
	return CheckPacketSizeWithMax(packet, maxSize)
}

// CheckPacketSizeWithMax checks if packet size exceeds a maximum.
func CheckPacketSizeWithMax(packet *packets.ControlPacket, maxSize int) error {
	packetSize, err := calculatePacketSize(packet)
	if err != nil {
		return fmt.Errorf("failed to calculate packet size: %w", err)
	}

	if packetSize > maxSize {
		return fmt.Errorf("%w: packet size %d exceeds maximum %d",
			ErrPacketOversize, packetSize, maxSize)
	}

	return nil
}

func calculatePacketSize(packet *packets.ControlPacket) (int, error) {
	encoded, err := wire.Encode(packet, wire.EncodeOptions{})
	if err != nil {
		return 0, err
	}
	return len(encoded), nil
}

// GetPacketSize returns packet size in bytes.
func GetPacketSize(packet *packets.ControlPacket) (int, error) {
	return calculatePacketSize(packet)
}

// CheckPublishPacketSize checks if PUBLISH packet exceeds maximum.
func CheckPublishPacketSize(publish *packets.Publish, maxSize int) error {
	return CheckPublishPacketSizeWithMax(publish, maxSize)
}

// CheckPublishPacketSizeWithMax checks if PUBLISH packet exceeds maximum.
func CheckPublishPacketSizeWithMax(publish *packets.Publish, maxSize int) error {
	return checkPacketContentSizeWithMax(packets.PUBLISH, publish, maxSize)
}

// CheckConnectPacketSize checks if CONNECT packet exceeds maximum.
func CheckConnectPacketSize(connect *packets.Connect, maxSize int) error {
	return CheckConnectPacketSizeWithMax(connect, maxSize)
}

// CheckConnectPacketSizeWithMax checks if CONNECT packet exceeds maximum.
func CheckConnectPacketSizeWithMax(connect *packets.Connect, maxSize int) error {
	return checkPacketContentSizeWithMax(packets.CONNECT, connect, maxSize)
}

// CheckSubscribePacketSize checks if SUBSCRIBE packet exceeds maximum.
func CheckSubscribePacketSize(subscribe *packets.Subscribe, maxSize int) error {
	return CheckSubscribePacketSizeWithMax(subscribe, maxSize)
}

// CheckSubscribePacketSizeWithMax checks if SUBSCRIBE packet exceeds maximum.
func CheckSubscribePacketSizeWithMax(subscribe *packets.Subscribe, maxSize int) error {
	return checkPacketContentSizeWithMax(packets.SUBSCRIBE, subscribe, maxSize)
}

func checkPacketContentSizeWithMax(packetType packets.PacketType, content packets.Packet, maxSize int) error {
	cp := packets.NewControlPacket(packetType)
	cp.Content = content
	return CheckPacketSizeWithMax(cp, maxSize)
}
