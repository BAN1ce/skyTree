package wire

import (
	"bytes"
	"io"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// maxRemainingLength 是 MQTT 剩余长度字段允许表示的最大值：256MB - 1。
const maxRemainingLength = 268435455

// validateFixedHeaderFlags 校验固定报头低 4 位是否符合各报文类型要求。
func validateFixedHeaderFlags(packetType mqtt5.PacketType, flags byte) error {
	switch packetType {
	case mqtt5.PUBLISH:
		qos := (flags >> 1) & 0x03
		dup := flags&0x08 != 0
		if qos == 0x03 {
			return malformed(packetType, "fixed header flags", "PUBLISH QoS 3 is reserved")
		}
		// MQTT 5.0: QoS 0 PUBLISH must not set DUP.
		if qos == 0x00 && dup {
			return malformed(packetType, "fixed header flags", "PUBLISH QoS 0 must not set DUP")
		}
		return nil
	case mqtt5.PUBREL, mqtt5.SUBSCRIBE, mqtt5.UNSUBSCRIBE:
		if flags == 0x02 {
			return nil
		}
	default:
		if flags == 0x00 {
			return nil
		}
	}
	return malformed(packetType, "fixed header flags", "invalid fixed header flags")
}

// fixedHeaderFlags 根据具体报文内容推导编码时应写入的固定报头标志位。
func fixedHeaderFlags(packet *mqtt5.ControlPacket, content mqtt5.Packet) (byte, error) {
	switch p := content.(type) {
	case *mqtt5.Publish:
		if p.QoS == 3 {
			return 0, malformed(mqtt5.PUBLISH, "qos", "PUBLISH QoS 3 is reserved")
		}
		if p.QoS == 0 && p.Duplicate {
			return 0, malformed(mqtt5.PUBLISH, "fixed header flags", "PUBLISH QoS 0 must not set DUP")
		}
		flags := p.QoS << 1
		if p.Duplicate {
			flags |= 0x08
		}
		if p.Retain {
			flags |= 0x01
		}
		return flags, nil
	case *mqtt5.Pubrel, *mqtt5.Subscribe, *mqtt5.Unsubscribe:
		return 0x02, nil
	default:
		return 0x00, nil
	}
}

// readRemainingLength 读取 MQTT Variable Byte Integer 格式的剩余长度。
func readRemainingLength(r io.Reader) (int, int, error) {
	var (
		value      int
		multiplier = 1
		digit      [1]byte
	)
	for width := 1; width <= 4; width++ {
		if _, err := io.ReadFull(r, digit[:]); err != nil {
			return 0, width - 1, err
		}
		value += int(digit[0]&127) * multiplier
		if value > maxRemainingLength {
			return 0, width, malformed(0, "remaining length", "remaining length exceeds MQTT maximum")
		}
		if digit[0]&128 == 0 {
			if width != minimalVBIBytes(value) {
				return 0, width, malformed(0, "remaining length", "remaining length is not minimally encoded")
			}
			return value, width, nil
		}
		multiplier *= 128
	}
	return 0, 4, malformed(0, "remaining length", "malformed variable byte integer")
}

// encodeRemainingLength 将剩余长度编码为 MQTT Variable Byte Integer。
func encodeRemainingLength(length int) ([]byte, error) {
	if length < 0 || length > maxRemainingLength {
		return nil, malformed(0, "remaining length", "remaining length exceeds MQTT maximum")
	}
	var out bytes.Buffer
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		out.WriteByte(digit)
		if length == 0 {
			return out.Bytes(), nil
		}
	}
}
