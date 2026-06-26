package wire

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// cursor 在报文体字节切片上顺序读取字段，并保留当前报文类型用于错误定位。
type cursor struct {
	packet mqtt5.PacketType
	data   []byte
	pos    int
}

// newCursor 创建一个从 data 起点开始读取的报文体游标。
func newCursor(packet mqtt5.PacketType, data []byte) *cursor {
	return &cursor{packet: packet, data: data}
}

// remaining 返回尚未读取的字节数。
func (c *cursor) remaining() int {
	return len(c.data) - c.pos
}

// readByte 读取单字节字段。
func (c *cursor) readByte(field string) (byte, error) {
	if c.remaining() < 1 {
		return 0, malformed(c.packet, field, "field is truncated")
	}
	out := c.data[c.pos]
	c.pos++
	return out, nil
}

// readUint16 读取 MQTT 使用的网络字节序二字节整数。
func (c *cursor) readUint16(field string) (uint16, error) {
	b, err := c.readBytes(field, 2)
	if err != nil {
		return 0, err
	}
	return uint16(b[0])<<8 | uint16(b[1]), nil
}

// readUint32 读取 MQTT 使用的网络字节序四字节整数。
func (c *cursor) readUint32(field string) (uint32, error) {
	b, err := c.readBytes(field, 4)
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// readBytes 读取固定长度字节片，返回值引用原始报文体切片。
func (c *cursor) readBytes(field string, n int) ([]byte, error) {
	if n < 0 || c.remaining() < n {
		return nil, malformed(c.packet, field, "field is truncated")
	}
	out := c.data[c.pos : c.pos+n]
	c.pos += n
	return out, nil
}

// readBinary 读取二字节长度前缀的二进制数据，并复制出独立切片。
func (c *cursor) readBinary(field string) ([]byte, error) {
	n, err := c.readUint16(field + " length")
	if err != nil {
		return nil, err
	}
	out, err := c.readBytes(field, int(n))
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), out...), nil
}

// readString 读取 MQTT UTF-8 Encoded String，并校验 UTF-8 与 NUL 字符约束。
func (c *cursor) readString(field string) (string, error) {
	data, err := c.readBinary(field)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", malformed(c.packet, field, "malformed UTF-8 string")
	}
	out := string(data)
	if strings.ContainsRune(out, '\u0000') {
		return "", malformed(c.packet, field, "UTF-8 string contains null character")
	}
	return out, nil
}

// readVBI 读取 MQTT Variable Byte Integer，最多允许 4 个字节。
func (c *cursor) readVBI(field string) (int, error) {
	var (
		value      int
		multiplier = 1
	)
	for width := 1; width <= 4; width++ {
		digit, err := c.readByte(field)
		if err != nil {
			return 0, err
		}
		value += int(digit&127) * multiplier
		if value > maxRemainingLength {
			return 0, malformed(c.packet, field, "variable byte integer exceeds MQTT maximum")
		}
		if digit&128 == 0 {
			if width != minimalVBIBytes(value) {
				return 0, malformed(c.packet, field, "variable byte integer is not minimally encoded")
			}
			return value, nil
		}
		multiplier *= 128
	}
	return 0, malformed(c.packet, field, "malformed variable byte integer")
}

// writeUint16 按网络字节序写入二字节整数。
func writeUint16(buf *bytes.Buffer, value uint16) {
	buf.WriteByte(byte(value >> 8))
	buf.WriteByte(byte(value))
}

// writeUint32 按网络字节序写入四字节整数。
func writeUint32(buf *bytes.Buffer, value uint32) {
	buf.WriteByte(byte(value >> 24))
	buf.WriteByte(byte(value >> 16))
	buf.WriteByte(byte(value >> 8))
	buf.WriteByte(byte(value))
}

// writeBinary 写入二字节长度前缀的二进制数据。
func writeBinary(buf *bytes.Buffer, packet mqtt5.PacketType, field string, value []byte) error {
	if len(value) > 65535 {
		return malformed(packet, field, "binary data exceeds two-byte length prefix")
	}
	writeUint16(buf, uint16(len(value)))
	_, _ = buf.Write(value)
	return nil
}

// writeString 写入 MQTT UTF-8 Encoded String，并在写入前完成合法性校验。
func writeString(buf *bytes.Buffer, packet mqtt5.PacketType, field, value string) error {
	if len(value) > 65535 {
		return malformed(packet, field, "UTF-8 string exceeds two-byte length prefix")
	}
	if !utf8.ValidString(value) {
		return malformed(packet, field, "malformed UTF-8 string")
	}
	if strings.ContainsRune(value, '\u0000') {
		return malformed(packet, field, "UTF-8 string contains null character")
	}
	writeUint16(buf, uint16(len(value)))
	_, _ = io.WriteString(buf, value)
	return nil
}

// writeVBI 写入 MQTT Variable Byte Integer。
func writeVBI(buf *bytes.Buffer, packet mqtt5.PacketType, field string, value int) error {
	encoded, err := encodeRemainingLength(value)
	if err != nil {
		return malformed(packet, field, err.Error())
	}
	_, _ = buf.Write(encoded)
	return nil
}
