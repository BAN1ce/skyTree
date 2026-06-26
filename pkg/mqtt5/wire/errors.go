package wire

import (
	"errors"
	"fmt"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ErrorKind 标识 wire 层错误所属的大类，便于上层转换 MQTT 响应原因码。
type ErrorKind int

const (
	// ErrMalformedPacket 表示报文格式不符合 MQTT 编码规则。
	ErrMalformedPacket ErrorKind = iota + 1
	// ErrProtocolError 表示报文语义违反 MQTT 协议状态或取值约束。
	ErrProtocolError
	// ErrPacketTooLarge 表示完整报文大小超过调用方设置的限制。
	ErrPacketTooLarge
	// ErrUnsupportedProtocolVersion 表示 CONNECT 使用了不支持的协议版本。
	ErrUnsupportedProtocolVersion
	// ErrImplementation 表示调用方传入的内存对象不满足编码前置条件。
	ErrImplementation
)

// WireError 是 MQTT 5 编解码层返回的结构化错误。
type WireError struct {
	// Kind 是错误分类。
	Kind ErrorKind
	// Packet 是触发错误的 MQTT 控制报文类型。
	Packet mqtt5.PacketType
	// Field 是触发错误的协议字段名。
	Field string
	// Reason 是建议映射到 MQTT 响应报文中的原因码。
	Reason mqtt5.ReasonCode
	// Message 是面向日志和调试的错误描述。
	Message string
	// PacketSize 和 MaxPacketSize 仅在 ErrPacketTooLarge 时有意义。
	PacketSize    int
	MaxPacketSize int
	// Cause 保存底层读取或校验错误，支持 errors.Unwrap。
	Cause error
}

// Error 返回包含报文类型、字段和底层原因的错误文本。
func (e *WireError) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if msg == "" {
		msg = "mqtt wire error"
	}
	if e.Packet != 0 {
		msg = mqtt5.PacketTypeName(e.Packet) + " " + msg
	}
	if e.Field != "" {
		msg = msg + " (" + e.Field + ")"
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

// Unwrap 返回底层错误，便于 errors.Is / errors.As 继续匹配。
func (e *WireError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func malformed(packet mqtt5.PacketType, field, message string) *WireError {
	return wireError(ErrMalformedPacket, packet, field, message, nil)
}

func protocol(packet mqtt5.PacketType, field, message string) *WireError {
	return wireError(ErrProtocolError, packet, field, message, nil)
}

func tooLarge(packet mqtt5.PacketType, size, max int) *WireError {
	err := wireError(
		ErrPacketTooLarge,
		packet,
		"remaining length",
		fmt.Sprintf("packet size %d exceeds maximum %d", size, max),
		nil,
	)
	err.PacketSize = size
	err.MaxPacketSize = max
	return err
}

func unsupportedVersion(version byte) *WireError {
	return wireError(
		ErrUnsupportedProtocolVersion,
		mqtt5.CONNECT,
		"protocol version",
		fmt.Sprintf("unsupported protocol version %d", version),
		nil,
	)
}

func implementation(packet mqtt5.PacketType, field, message string) *WireError {
	return wireError(ErrImplementation, packet, field, message, nil)
}

func wrapMalformed(packet mqtt5.PacketType, field, message string, err error) *WireError {
	return wireError(ErrMalformedPacket, packet, field, message, err)
}

// wireError 统一补齐错误分类对应的 MQTT 5 原因码。
func wireError(kind ErrorKind, packet mqtt5.PacketType, field, message string, cause error) *WireError {
	reason := mqtt5.ReasonCode(0)
	switch kind {
	case ErrPacketTooLarge:
		reason = mqtt5.DisconnectPacketTooLarge
	case ErrUnsupportedProtocolVersion:
		reason = mqtt5.ConnAckUnsupportedProtocolVersion
	case ErrProtocolError:
		reason = mqtt5.DisconnectProtocolError
	case ErrMalformedPacket:
		reason = mqtt5.DisconnectMalformedPacket
	case ErrImplementation:
		reason = mqtt5.DisconnectImplementationSpecificError
	}
	return &WireError{
		Kind:    kind,
		Packet:  packet,
		Field:   field,
		Reason:  reason,
		Message: message,
		Cause:   cause,
	}
}

// classifyReadError 将 io 读取错误包装成协议可理解的 WireError。
func classifyReadError(packet mqtt5.PacketType, field string, err error) error {
	if err == nil {
		return nil
	}
	var wireErr *WireError
	if errors.As(err, &wireErr) {
		return wireErr
	}
	return wrapMalformed(packet, field, "failed to read field", err)
}
