package wire

import (
	"io"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// EncodeOptions 控制编码过程中的安全限制。
type EncodeOptions struct {
	// MaxPacketSize 为 0 时不限制；大于 0 时按完整 MQTT 报文长度校验。
	MaxPacketSize int
}

// Writer 封装一个 io.Writer，用于写出 MQTT 5 控制报文。
type Writer struct {
	w    io.Writer
	opts EncodeOptions
}

// NewWriter 创建一个带编码选项的 MQTT 5 Writer。
func NewWriter(w io.Writer, opts EncodeOptions) *Writer {
	return &Writer{w: w, opts: opts}
}

// Write 将控制报文编码后写入底层 writer。
func (w *Writer) Write(packet *mqtt5.ControlPacket) (int64, error) {
	if w == nil {
		return 0, implementation(0, "writer", "nil writer")
	}
	return Write(w.w, packet, w.opts)
}

// Write 将控制报文编码并写入指定 io.Writer。
func Write(w io.Writer, packet *mqtt5.ControlPacket, opts EncodeOptions) (int64, error) {
	encoded, err := Encode(packet, opts)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(encoded)
	return int64(n), err
}

// Encode 将控制报文编码成完整 MQTT 字节流，包括固定报头和剩余长度。
func Encode(packet *mqtt5.ControlPacket, opts EncodeOptions) ([]byte, error) {
	if packet == nil {
		return nil, implementation(0, "packet", "nil control packet")
	}
	content := packet.CanonicalPacket()
	if content == nil {
		return nil, implementation(packet.Type, "content", "nil packet content")
	}
	packetType := content.PacketType()
	if packet.Type != 0 && packet.Type != packetType {
		return nil, implementation(packetType, "type", "control packet type does not match content")
	}

	flags, err := fixedHeaderFlags(packet, content)
	if err != nil {
		return nil, err
	}
	if err := validateFixedHeaderFlags(packetType, flags); err != nil {
		return nil, err
	}

	body, err := encodePacket(content)
	if err != nil {
		return nil, err
	}
	if len(body) > maxRemainingLength {
		return nil, malformed(packetType, "remaining length", "remaining length exceeds MQTT maximum")
	}
	remainingLength, err := encodeRemainingLength(len(body))
	if err != nil {
		return nil, err
	}

	totalSize := 1 + len(remainingLength) + len(body)
	if opts.MaxPacketSize > 0 && totalSize > opts.MaxPacketSize {
		return nil, tooLarge(packetType, totalSize, opts.MaxPacketSize)
	}

	out := make([]byte, 0, totalSize)
	out = append(out, byte(packetType)<<4|flags)
	out = append(out, remainingLength...)
	out = append(out, body...)
	return out, nil
}
