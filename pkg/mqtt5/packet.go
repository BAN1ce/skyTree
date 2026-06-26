// Package mqtt5 定义 MQTT 5 控制报文、属性和原因码的内存模型。
package mqtt5

import (
	"fmt"
	"strings"
)

// PacketType 表示 MQTT 固定报头高 4 位中的控制报文类型。
type PacketType = byte

const (
	_ PacketType = iota
	// CONNECT 是客户端建立 MQTT 会话的请求报文。
	CONNECT
	// CONNACK 是服务端对 CONNECT 的连接确认报文。
	CONNACK
	// PUBLISH 承载应用层消息。
	PUBLISH
	// PUBACK 是 QoS 1 发布流程的确认报文。
	PUBACK
	// PUBREC 是 QoS 2 发布流程的第一阶段确认报文。
	PUBREC
	// PUBREL 是 QoS 2 发布流程的第二阶段释放报文。
	PUBREL
	// PUBCOMP 是 QoS 2 发布流程的完成报文。
	PUBCOMP
	// SUBSCRIBE 是客户端订阅主题过滤器的请求报文。
	SUBSCRIBE
	// SUBACK 是服务端对 SUBSCRIBE 的订阅确认报文。
	SUBACK
	// UNSUBSCRIBE 是客户端取消订阅主题过滤器的请求报文。
	UNSUBSCRIBE
	// UNSUBACK 是服务端对 UNSUBSCRIBE 的取消订阅确认报文。
	UNSUBACK
	// PINGREQ 是客户端发送的心跳请求报文。
	PINGREQ
	// PINGRESP 是服务端返回的心跳响应报文。
	PINGRESP
	// DISCONNECT 表示连接即将关闭，并可携带关闭原因。
	DISCONNECT
	// AUTH 用于 MQTT 5 扩展认证流程。
	AUTH
	// WILLPROPERTIES 不是独立控制报文，只用于遗嘱属性的编码校验上下文。
	WILLPROPERTIES
)

// ControlPacket 把固定报头和具体报文内容组合在一起。
type ControlPacket struct {
	// Content 是当前实现使用的规范内容字段。
	Content Packet
	// Packet 保留为兼容旧调用方的别名，始终与 Content 同步。
	Packet Packet
	FixedHeader
}

// FixedHeader 保存 MQTT 固定报头中解析出来的类型、标志位和剩余长度。
type FixedHeader struct {
	Type            PacketType
	Flags           byte
	RemainingLength int
}

// Packet 是所有 MQTT 5 控制报文都要实现的最小接口。
type Packet interface {
	PacketType() PacketType
}

// Optional 表示某个值是否在报文中显式出现。
type Optional[T any] struct {
	Value T
	Set   bool
}

// NewControlPacket 按报文类型创建带默认属性结构的控制报文。
func NewControlPacket(packetType PacketType) *ControlPacket {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: packetType}}
	switch packetType {
	case CONNECT:
		cp.setPacket(&Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			Properties:      &ConnectProperties{},
		})
	case CONNACK:
		cp.setPacket(&ConnAck{Properties: &ConnAckProperties{}})
	case PUBLISH:
		cp.setPacket(&Publish{Properties: &PublishProperties{}})
	case PUBACK:
		cp.setPacket(&Puback{Properties: &PubackProperties{}})
	case PUBREC:
		cp.setPacket(&Pubrec{Properties: &PubrecProperties{}})
	case PUBREL:
		cp.Flags = 2
		cp.setPacket(&Pubrel{Properties: &PubrelProperties{}})
	case PUBCOMP:
		cp.setPacket(&Pubcomp{Properties: &PubcompProperties{}})
	case SUBSCRIBE:
		cp.Flags = 2
		cp.setPacket(&Subscribe{
			Properties:    &SubscribeProperties{},
			Subscriptions: []SubOptions{},
		})
	case SUBACK:
		cp.setPacket(&Suback{Properties: &SubackProperties{}})
	case UNSUBSCRIBE:
		cp.Flags = 2
		cp.setPacket(&Unsubscribe{Properties: &UnsubscribeProperties{}})
	case UNSUBACK:
		cp.setPacket(&Unsuback{Properties: &UnsubackProperties{}})
	case PINGREQ:
		cp.setPacket(&Pingreq{})
	case PINGRESP:
		cp.setPacket(&Pingresp{})
	case DISCONNECT:
		cp.setPacket(&Disconnect{Properties: &DisconnectProperties{}})
	case AUTH:
		cp.setPacket(&Auth{Properties: &AuthProperties{}})
	default:
		return nil
	}
	return cp
}

// CanonicalPacket 返回控制报文实际承载的内容。
func (c *ControlPacket) CanonicalPacket() Packet {
	if c == nil {
		return nil
	}
	if c.Content != nil {
		return c.Content
	}
	return c.Packet
}

// SetPacket 设置控制报文内容，并同步固定报头类型和默认标志位。
func (c *ControlPacket) SetPacket(packet Packet) {
	c.setPacket(packet)
}

func (c *ControlPacket) setPacket(packet Packet) {
	c.Content = packet
	c.Packet = packet
	if packet != nil {
		c.Type = packet.PacketType()
		if c.Type == PUBREL || c.Type == SUBSCRIBE || c.Type == UNSUBSCRIBE {
			c.Flags = 2
		}
	}
}

// PacketID 返回带报文标识符的控制报文中的 Packet Identifier。
func (c *ControlPacket) PacketID() uint16 {
	switch p := c.CanonicalPacket().(type) {
	case *Publish:
		return p.PacketID
	case *Puback:
		return p.PacketID
	case *Pubrec:
		return p.PacketID
	case *Pubrel:
		return p.PacketID
	case *Pubcomp:
		return p.PacketID
	case *Subscribe:
		return p.PacketID
	case *Suback:
		return p.PacketID
	case *Unsubscribe:
		return p.PacketID
	case *Unsuback:
		return p.PacketID
	default:
		return 0
	}
}

// PacketType 返回控制报文类型的人类可读名称。
func (c *ControlPacket) PacketType() string {
	if c == nil {
		return ""
	}
	return PacketTypeName(c.Type)
}

// String 返回控制报文内容的调试字符串。
func (c *ControlPacket) String() string {
	if c == nil {
		return "<nil>"
	}
	if p := c.CanonicalPacket(); p != nil {
		return fmt.Sprint(p)
	}
	return fmt.Sprintf("Unknown packet type: %d", c.Type)
}

// PacketTypeName 把 MQTT 报文类型编号转换成固定的大写名称。
func PacketTypeName(packetType PacketType) string {
	if int(packetType) >= 0 && int(packetType) < len(packetNames) {
		return packetNames[packetType]
	}
	return "UNKNOWN"
}

var packetNames = [...]string{
	"",
	"CONNECT",
	"CONNACK",
	"PUBLISH",
	"PUBACK",
	"PUBREC",
	"PUBREL",
	"PUBCOMP",
	"SUBSCRIBE",
	"SUBACK",
	"UNSUBSCRIBE",
	"UNSUBACK",
	"PINGREQ",
	"PINGRESP",
	"DISCONNECT",
	"AUTH",
}

// Connect 表示 MQTT 5 CONNECT 报文的可变报头和载荷。
type Connect struct {
	// WillMessage 是客户端异常断开时由服务端发布的遗嘱消息体。
	WillMessage []byte
	// Password 是 CONNECT 载荷中的二进制密码字段。
	Password []byte
	// Username 是 CONNECT 载荷中的用户名字段。
	Username string
	// ProtocolName 固定为 "MQTT"。
	ProtocolName string
	// ClientID 是客户端标识符。
	ClientID string
	// WillTopic 是遗嘱消息要发布到的主题名。
	WillTopic string
	// Properties 保存 CONNECT 可变报头中的 MQTT 5 属性。
	Properties *ConnectProperties
	// WillProperties 保存遗嘱消息专属属性。
	WillProperties *PublishProperties
	// KeepAlive 是客户端声明的心跳间隔，单位为秒。
	KeepAlive uint16
	// ProtocolVersion 固定为 5。
	ProtocolVersion byte
	// WillQOS 是遗嘱消息的 QoS 等级。
	WillQOS byte
	// PasswordFlag 表示载荷中是否包含密码字段。
	PasswordFlag bool
	// UsernameFlag 表示载荷中是否包含用户名字段。
	UsernameFlag bool
	// WillRetain 表示遗嘱消息是否作为保留消息发布。
	WillRetain bool
	// WillFlag 表示 CONNECT 是否携带遗嘱消息。
	WillFlag bool
	// CleanStart 表示是否要求服务端创建全新的会话状态。
	CleanStart bool
}

func (*Connect) PacketType() PacketType { return CONNECT }

// PackFlags 按 MQTT CONNECT 标志位布局打包用户名、密码、遗嘱和 Clean Start。
func (c *Connect) PackFlags() byte {
	var flags byte
	if c.UsernameFlag {
		flags |= 0x80
	}
	if c.PasswordFlag {
		flags |= 0x40
	}
	if c.WillFlag {
		flags |= 0x04
		flags |= c.WillQOS << 3
		if c.WillRetain {
			flags |= 0x20
		}
	}
	if c.CleanStart {
		flags |= 0x02
	}
	return flags
}

// String 返回 CONNECT 报文的调试字符串，密码以十六进制输出。
func (c *Connect) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "CONNECT: ProtocolName:%s ProtocolVersion:%d ClientID:%s KeepAlive:%d CleanStart:%t", c.ProtocolName, c.ProtocolVersion, c.ClientID, c.KeepAlive, c.CleanStart)
	if c.UsernameFlag {
		fmt.Fprintf(&b, " Username:%s", c.Username)
	}
	if c.PasswordFlag {
		fmt.Fprintf(&b, " Password:%X", c.Password)
	}
	if c.WillFlag {
		fmt.Fprintf(&b, " WillTopic:%s WillQOS:%d WillRetain:%t", c.WillTopic, c.WillQOS, c.WillRetain)
	}
	return b.String()
}

// ConnAck 表示 MQTT 5 CONNACK 连接确认报文。
type ConnAck struct {
	Properties     *ConnAckProperties
	ReasonCode     byte
	SessionPresent bool
}

func (*ConnAck) PacketType() PacketType { return CONNACK }

// String 返回 CONNACK 报文的调试字符串。
func (c *ConnAck) String() string {
	return fmt.Sprintf("CONNACK: ReasonCode:%d SessionPresent:%t Properties:\n%s", c.ReasonCode, c.SessionPresent, c.Properties)
}

// Publish 表示 MQTT 5 PUBLISH 应用消息报文。
type Publish struct {
	// Payload 是应用消息载荷，编码时原样写入报文体。
	Payload    []byte
	Topic      string
	Properties *PublishProperties
	PacketID   uint16
	QoS        byte
	Duplicate  bool
	Retain     bool
}

func (*Publish) PacketType() PacketType { return PUBLISH }

// SetIdentifier 设置 QoS 1/2 PUBLISH 所需的报文标识符。
func (p *Publish) SetIdentifier(packetID uint16) {
	p.PacketID = packetID
}

// Type 返回与旧接口兼容的 PUBLISH 类型编号。
func (*Publish) Type() byte { return PUBLISH }

// ToControlPacket 根据 PUBLISH 的 QoS、DUP、Retain 状态生成完整控制报文。
func (p *Publish) ToControlPacket() *ControlPacket {
	flags := p.QoS << 1
	if p.Duplicate {
		flags |= 0x08
	}
	if p.Retain {
		flags |= 0x01
	}
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBLISH, Flags: flags}}
	cp.setPacket(p)
	cp.Flags = flags
	return cp
}

// String 返回 PUBLISH 报文的调试字符串。
func (p *Publish) String() string {
	return fmt.Sprintf(
		"PUBLISH: PacketID:%d QOS:%d Topic:%s Duplicate:%t Retain:%t Payload:%s Properties:\n%s",
		p.PacketID,
		p.QoS,
		p.Topic,
		p.Duplicate,
		p.Retain,
		string(p.Payload),
		p.Properties,
	)
}

// Puback 表示 QoS 1 PUBLISH 的确认报文。
type Puback struct {
	Properties *PubackProperties
	PacketID   uint16
	ReasonCode byte
}

func (*Puback) PacketType() PacketType { return PUBACK }

// String 返回 PUBACK 报文的调试字符串。
func (p *Puback) String() string {
	return fmt.Sprintf("PUBACK: PacketID:%d ReasonCode:%X Properties:\n%s", p.PacketID, p.ReasonCode, p.Properties)
}

// Reason 返回 PUBACK 原因码对应的英文描述。
func (p *Puback) Reason() string {
	return reasonString(p.ReasonCode, map[byte]string{
		PubackSuccess:                     "Success",
		PubackNoMatchingSubscribers:       "No matching subscribers",
		PubackUnspecifiedError:            "Unspecified error",
		PubackImplementationSpecificError: "Implementation specific error",
		PubackNotAuthorized:               "Not authorized",
		PubackTopicNameInvalid:            "Topic Name invalid",
		PubackPacketIdentifierInUse:       "Packet Identifier in use",
		PubackQuotaExceeded:               "Quota exceeded",
		PubackPayloadFormatInvalid:        "Payload format invalid",
		PubackRetainNotSupported:          "Retain not supported",
		PubackQoSNotSupported:             "QoS not supported",
	})
}

// Pubrec 表示 QoS 2 PUBLISH 的第一阶段确认报文。
type Pubrec struct {
	Properties *PubrecProperties
	PacketID   uint16
	ReasonCode byte
}

func (*Pubrec) PacketType() PacketType { return PUBREC }

// ToControlPacket 生成 PUBREC 对应的完整控制报文。
func (p *Pubrec) ToControlPacket() *ControlPacket {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBREC}}
	cp.setPacket(p)
	return cp
}

// String 返回 PUBREC 报文的调试字符串。
func (p *Pubrec) String() string {
	return fmt.Sprintf("PUBREC: PacketID:%d ReasonCode:%X Properties:\n%s", p.PacketID, p.ReasonCode, p.Properties)
}

// Reason 返回 PUBREC 原因码对应的英文描述。
func (p *Pubrec) Reason() string {
	return reasonString(p.ReasonCode, map[byte]string{
		PubrecSuccess:                     "Success",
		PubrecNoMatchingSubscribers:       "No matching subscribers",
		PubrecUnspecifiedError:            "Unspecified error",
		PubrecImplementationSpecificError: "Implementation specific error",
		PubrecNotAuthorized:               "Not authorized",
		PubrecTopicNameInvalid:            "Topic Name invalid",
		PubrecPacketIdentifierInUse:       "Packet Identifier in use",
		PubrecQuotaExceeded:               "Quota exceeded",
		PubrecPayloadFormatInvalid:        "Payload format invalid",
		PubrecRetainNotSupported:          "Retain not supported",
		PubrecQoSNotSupported:             "QoS not supported",
	})
}

// Pubrel 表示 QoS 2 发布流程中的释放报文。
type Pubrel struct {
	Properties *PubrelProperties
	PacketID   uint16
	ReasonCode byte
}

func (*Pubrel) PacketType() PacketType { return PUBREL }

// String 返回 PUBREL 报文的调试字符串。
func (p *Pubrel) String() string {
	return fmt.Sprintf("PUBREL: PacketID:%d ReasonCode:%X Properties:\n%s", p.PacketID, p.ReasonCode, p.Properties)
}

// Pubcomp 表示 QoS 2 发布流程的完成报文。
type Pubcomp struct {
	Properties *PubcompProperties
	PacketID   uint16
	ReasonCode byte
}

func (*Pubcomp) PacketType() PacketType { return PUBCOMP }

// String 返回 PUBCOMP 报文的调试字符串。
func (p *Pubcomp) String() string {
	return fmt.Sprintf("PUBCOMP: PacketID:%d ReasonCode:%X Properties:\n%s", p.PacketID, p.ReasonCode, p.Properties)
}

// Subscribe 表示 MQTT 5 SUBSCRIBE 订阅请求报文。
type Subscribe struct {
	Properties    *SubscribeProperties
	Subscriptions []SubOptions
	PacketID      uint16
}

func (*Subscribe) PacketType() PacketType { return SUBSCRIBE }

// SetIdentifier 设置 SUBSCRIBE 报文标识符。
func (s *Subscribe) SetIdentifier(packetID uint16) {
	s.PacketID = packetID
}

// Type 返回与旧接口兼容的 SUBSCRIBE 类型编号。
func (*Subscribe) Type() byte { return SUBSCRIBE }

// String 返回 SUBSCRIBE 报文的调试字符串。
func (s *Subscribe) String() string {
	return fmt.Sprintf("SUBSCRIBE: PacketID:%d Subscriptions:%v Properties:\n%s", s.PacketID, s.Subscriptions, s.Properties)
}

const (
	// RetainSendOnSubscribe 表示每次订阅时都发送保留消息。
	RetainSendOnSubscribe = iota
	// RetainSendOnSubscribeIfNew 表示只有新建订阅时才发送保留消息。
	RetainSendOnSubscribeIfNew
	// RetainDoNotSend 表示订阅建立时不发送保留消息。
	RetainDoNotSend
)

// SubOptions 表示单个主题过滤器对应的订阅选项。
type SubOptions struct {
	Topic             string
	QoS               byte
	RetainHandling    byte
	NoLocal           bool
	RetainAsPublished bool
}

// Pack 按 MQTT 订阅选项位布局编码 QoS、No Local、Retain As Published 和 Retain Handling。
func (s *SubOptions) Pack() byte {
	var out byte
	out |= s.QoS & 0x03
	if s.NoLocal {
		out |= 0x04
	}
	if s.RetainAsPublished {
		out |= 0x08
	}
	out |= (s.RetainHandling << 4) & 0x30
	return out
}

// Suback 表示 MQTT 5 SUBACK 订阅确认报文。
type Suback struct {
	Properties *SubackProperties
	Reasons    []byte
	PacketID   uint16
}

func (*Suback) PacketType() PacketType { return SUBACK }

// String 返回 SUBACK 报文的调试字符串。
func (s *Suback) String() string {
	return fmt.Sprintf("SUBACK: PacketID:%d ReasonCode:%v Properties:\n%s", s.PacketID, s.Reasons, s.Properties)
}

// Unsubscribe 表示 MQTT 5 UNSUBSCRIBE 取消订阅请求报文。
type Unsubscribe struct {
	Topics     []string
	Properties *UnsubscribeProperties
	PacketID   uint16
}

func (*Unsubscribe) PacketType() PacketType { return UNSUBSCRIBE }

// SetIdentifier 设置 UNSUBSCRIBE 报文标识符。
func (u *Unsubscribe) SetIdentifier(packetID uint16) {
	u.PacketID = packetID
}

// Type 返回与旧接口兼容的 UNSUBSCRIBE 类型编号。
func (*Unsubscribe) Type() byte { return UNSUBSCRIBE }

// String 返回 UNSUBSCRIBE 报文的调试字符串。
func (u *Unsubscribe) String() string {
	return fmt.Sprintf("UNSUBSCRIBE: PacketID:%d Topics:%v Properties:\n%s", u.PacketID, u.Topics, u.Properties)
}

// Unsuback 表示 MQTT 5 UNSUBACK 取消订阅确认报文。
type Unsuback struct {
	Reasons    []byte
	Properties *UnsubackProperties
	PacketID   uint16
}

func (*Unsuback) PacketType() PacketType { return UNSUBACK }

// String 返回 UNSUBACK 报文的调试字符串。
func (u *Unsuback) String() string {
	return fmt.Sprintf("UNSUBACK: PacketID:%d ReasonCode:%v Properties:\n%s", u.PacketID, u.Reasons, u.Properties)
}

// Pingreq 表示 MQTT 心跳请求报文。
type Pingreq struct{}

func (*Pingreq) PacketType() PacketType { return PINGREQ }

// String 返回 PINGREQ 的调试字符串。
func (*Pingreq) String() string { return "PINGREQ" }

// Pingresp 表示 MQTT 心跳响应报文。
type Pingresp struct{}

func (*Pingresp) PacketType() PacketType { return PINGRESP }

// String 返回 PINGRESP 的调试字符串。
func (*Pingresp) String() string { return "PINGRESP" }

// Disconnect 表示 MQTT 5 DISCONNECT 断开连接报文。
type Disconnect struct {
	Properties *DisconnectProperties
	ReasonCode byte
}

func (*Disconnect) PacketType() PacketType { return DISCONNECT }

// String 返回 DISCONNECT 报文的调试字符串。
func (d *Disconnect) String() string {
	return fmt.Sprintf("DISCONNECT: ReasonCode:%X Properties:\n%s", d.ReasonCode, d.Properties)
}

// Auth 表示 MQTT 5 AUTH 扩展认证报文。
type Auth struct {
	Properties *AuthProperties
	ReasonCode byte
}

func (*Auth) PacketType() PacketType { return AUTH }

// String 返回 AUTH 报文的调试字符串。
func (a *Auth) String() string {
	return fmt.Sprintf("AUTH: ReasonCode:%X Properties:\n%s", a.ReasonCode, a.Properties)
}

func reasonString(code byte, values map[byte]string) string {
	return values[code]
}
