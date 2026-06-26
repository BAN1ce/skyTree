// Package wire 负责 MQTT 5 控制报文与网络字节流之间的编解码。
package wire

import (
	"io"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// DecodeOptions 控制解码过程中的安全限制。
type DecodeOptions struct {
	// MaxPacketSize 为 0 时不限制；大于 0 时按完整 MQTT 报文长度校验。
	MaxPacketSize int
}

// Reader 封装一个 io.Reader，用于连续解码 MQTT 5 控制报文。
type Reader struct {
	r    io.Reader
	opts DecodeOptions
}

// NewReader 创建一个带解码选项的 MQTT 5 Reader。
func NewReader(r io.Reader, opts DecodeOptions) *Reader {
	return &Reader{r: r, opts: opts}
}

// Decode 从底层 reader 解码下一帧 MQTT 5 控制报文。
func (r *Reader) Decode() (*mqtt5.ControlPacket, error) {
	if r == nil {
		return nil, implementation(0, "reader", "nil reader")
	}
	return Decode(r.r, r.opts)
}

// Decode 从 io.Reader 读取固定报头、剩余长度和报文体并解析成控制报文。
func Decode(r io.Reader, opts DecodeOptions) (*mqtt5.ControlPacket, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return nil, err
	}

	packetType := mqtt5.PacketType(first[0] >> 4)
	flags := first[0] & 0x0f
	if packetType < mqtt5.CONNECT || packetType > mqtt5.AUTH {
		return nil, malformed(packetType, "packet type", "unknown packet type")
	}
	if err := validateFixedHeaderFlags(packetType, flags); err != nil {
		return nil, err
	}

	remainingLength, remainingLengthWidth, err := readRemainingLength(r)
	if err != nil {
		return nil, classifyReadError(packetType, "remaining length", err)
	}
	totalSize := 1 + remainingLengthWidth + remainingLength
	if opts.MaxPacketSize > 0 && totalSize > opts.MaxPacketSize {
		return nil, tooLarge(packetType, totalSize, opts.MaxPacketSize)
	}

	body := make([]byte, remainingLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, classifyReadError(packetType, "body", err)
	}

	content, err := decodePacket(packetType, flags, body)
	if err != nil {
		return nil, err
	}

	cp := &mqtt5.ControlPacket{
		FixedHeader: mqtt5.FixedHeader{
			Type:            packetType,
			Flags:           flags,
			RemainingLength: remainingLength,
		},
		Content: content,
		Packet:  content,
	}
	return cp, nil
}
