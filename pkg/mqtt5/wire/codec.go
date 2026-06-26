package wire

import (
	"bytes"
	"strings"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
)

// decodePacket 根据固定报头中的报文类型分派到具体解码器。
func decodePacket(packetType mqtt5.PacketType, flags byte, body []byte) (mqtt5.Packet, error) {
	cur := newCursor(packetType, body)
	var (
		packet mqtt5.Packet
		err    error
	)
	switch packetType {
	case mqtt5.CONNECT:
		packet, err = decodeConnect(cur)
	case mqtt5.CONNACK:
		packet, err = decodeConnack(cur)
	case mqtt5.PUBLISH:
		packet, err = decodePublish(cur, flags)
	case mqtt5.PUBACK:
		packet, err = decodeAck(cur, packetType, &mqtt5.Puback{Properties: &mqtt5.PubackProperties{}})
	case mqtt5.PUBREC:
		packet, err = decodeAck(cur, packetType, &mqtt5.Pubrec{Properties: &mqtt5.PubrecProperties{}})
	case mqtt5.PUBREL:
		packet, err = decodeAck(cur, packetType, &mqtt5.Pubrel{Properties: &mqtt5.PubrelProperties{}})
	case mqtt5.PUBCOMP:
		packet, err = decodeAck(cur, packetType, &mqtt5.Pubcomp{Properties: &mqtt5.PubcompProperties{}})
	case mqtt5.SUBSCRIBE:
		packet, err = decodeSubscribe(cur)
	case mqtt5.SUBACK:
		packet, err = decodeSuback(cur)
	case mqtt5.UNSUBSCRIBE:
		packet, err = decodeUnsubscribe(cur)
	case mqtt5.UNSUBACK:
		packet, err = decodeUnsuback(cur)
	case mqtt5.PINGREQ:
		packet, err = decodeEmpty(cur, &mqtt5.Pingreq{})
	case mqtt5.PINGRESP:
		packet, err = decodeEmpty(cur, &mqtt5.Pingresp{})
	case mqtt5.DISCONNECT:
		packet, err = decodeDisconnect(cur)
	case mqtt5.AUTH:
		packet, err = decodeAuth(cur)
	default:
		err = malformed(packetType, "packet type", "unknown packet type")
	}
	if err != nil {
		return nil, err
	}
	if cur.remaining() != 0 {
		return nil, malformed(packetType, "body", "packet body has trailing bytes")
	}
	return packet, nil
}

// decodeConnect 解析 CONNECT 的可变报头、属性和载荷字段。
func decodeConnect(cur *cursor) (*mqtt5.Connect, error) {
	out := &mqtt5.Connect{Properties: &mqtt5.ConnectProperties{}}
	var err error
	out.ProtocolName, err = cur.readString("protocol name")
	if err != nil {
		return nil, err
	}
	if out.ProtocolName != "MQTT" {
		return nil, malformed(mqtt5.CONNECT, "protocol name", "protocol name must be MQTT")
	}
	out.ProtocolVersion, err = cur.readByte("protocol version")
	if err != nil {
		return nil, err
	}
	if out.ProtocolVersion != 5 {
		return nil, unsupportedVersion(out.ProtocolVersion)
	}
	flags, err := cur.readByte("connect flags")
	if err != nil {
		return nil, err
	}
	if err := unpackConnectFlags(out, flags); err != nil {
		return nil, err
	}
	out.KeepAlive, err = cur.readUint16("keep alive")
	if err != nil {
		return nil, err
	}
	if err := decodeProperties(cur, mqtt5.CONNECT, out.Properties); err != nil {
		return nil, err
	}
	out.ClientID, err = cur.readString("client id")
	if err != nil {
		return nil, err
	}
	if out.WillFlag {
		out.WillProperties = &mqtt5.PublishProperties{}
		if err := decodeProperties(cur, mqtt5.WILLPROPERTIES, out.WillProperties); err != nil {
			return nil, err
		}
		out.WillTopic, err = cur.readString("will topic")
		if err != nil {
			return nil, err
		}
		if err := validateTopicName(mqtt5.CONNECT, "will topic", out.WillTopic, false); err != nil {
			return nil, err
		}
		out.WillMessage, err = cur.readBinary("will message")
		if err != nil {
			return nil, err
		}
	}
	if out.UsernameFlag {
		out.Username, err = cur.readString("username")
		if err != nil {
			return nil, err
		}
	}
	if out.PasswordFlag {
		out.Password, err = cur.readBinary("password")
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// unpackConnectFlags 将 CONNECT 标志位拆成结构体字段并校验保留位。
func unpackConnectFlags(out *mqtt5.Connect, flags byte) error {
	if flags&0x01 != 0 {
		return malformed(mqtt5.CONNECT, "connect flags", "reserved bit must be zero")
	}
	out.CleanStart = flags&0x02 != 0
	out.WillFlag = flags&0x04 != 0
	out.WillQOS = (flags >> 3) & 0x03
	out.WillRetain = flags&0x20 != 0
	out.PasswordFlag = flags&0x40 != 0
	out.UsernameFlag = flags&0x80 != 0
	if !out.WillFlag {
		if out.WillQOS != 0 {
			return malformed(mqtt5.CONNECT, "connect flags", "will QoS must be zero when will flag is false")
		}
		if out.WillRetain {
			return malformed(mqtt5.CONNECT, "connect flags", "will retain must be false when will flag is false")
		}
	}
	if out.WillQOS == 3 {
		return malformed(mqtt5.CONNECT, "connect flags", "will QoS 3 is reserved")
	}
	// MQTT 5 / 3.1.1 compatibility rule: Password Flag cannot be set when Username Flag is unset.
	if out.PasswordFlag && !out.UsernameFlag {
		return malformed(mqtt5.CONNECT, "connect flags", "password flag requires username flag")
	}
	return nil
}

// decodeConnack 解析服务端连接确认报文，并校验 Session Present 规则。
func decodeConnack(cur *cursor) (*mqtt5.ConnAck, error) {
	out := &mqtt5.ConnAck{Properties: &mqtt5.ConnAckProperties{}}
	flags, err := cur.readByte("acknowledge flags")
	if err != nil {
		return nil, err
	}
	if flags&0xfe != 0 {
		return nil, malformed(mqtt5.CONNACK, "acknowledge flags", "reserved bits must be zero")
	}
	out.SessionPresent = flags&0x01 != 0
	out.ReasonCode, err = cur.readByte("reason code")
	if err != nil {
		return nil, err
	}
	if err := validateReason(mqtt5.CONNACK, out.ReasonCode); err != nil {
		return nil, err
	}
	if out.ReasonCode != mqtt5.ConnAckSuccess && out.SessionPresent {
		return nil, malformed(mqtt5.CONNACK, "session present", "session present must be false for unsuccessful CONNACK")
	}
	if err := decodeProperties(cur, mqtt5.CONNACK, out.Properties); err != nil {
		return nil, err
	}
	return out, nil
}

// decodePublish 解析 PUBLISH 报文，按 QoS 决定是否读取 Packet Identifier。
func decodePublish(cur *cursor, flags byte) (*mqtt5.Publish, error) {
	out := &mqtt5.Publish{
		Duplicate:  flags&0x08 != 0,
		QoS:        (flags >> 1) & 0x03,
		Retain:     flags&0x01 != 0,
		Properties: &mqtt5.PublishProperties{},
	}
	if out.QoS == 3 {
		return nil, malformed(mqtt5.PUBLISH, "qos", "PUBLISH QoS 3 is reserved")
	}
	var err error
	out.Topic, err = cur.readString("topic name")
	if err != nil {
		return nil, err
	}
	if out.QoS > 0 {
		out.PacketID, err = cur.readUint16("packet identifier")
		if err != nil {
			return nil, err
		}
		if out.PacketID == 0 {
			return nil, malformed(mqtt5.PUBLISH, "packet identifier", "packet identifier must be non-zero")
		}
	}
	if err := decodeProperties(cur, mqtt5.PUBLISH, out.Properties); err != nil {
		return nil, err
	}
	if out.Topic == "" {
		if out.Properties.TopicAlias == nil {
			return nil, malformed(mqtt5.PUBLISH, "topic name", "topic name is required unless topic alias is present")
		}
	} else if err := validateTopicName(mqtt5.PUBLISH, "topic name", out.Topic, false); err != nil {
		return nil, err
	}
	out.Payload = append([]byte(nil), cur.data[cur.pos:]...)
	cur.pos = len(cur.data)
	return out, nil
}

// decodeAck 解析 PUBACK/PUBREC/PUBREL/PUBCOMP 的共同报文布局。
func decodeAck(cur *cursor, packetType mqtt5.PacketType, out mqtt5.Packet) (mqtt5.Packet, error) {
	packetID, err := cur.readUint16("packet identifier")
	if err != nil {
		return nil, err
	}
	if packetID == 0 {
		return nil, malformed(packetType, "packet identifier", "packet identifier must be non-zero")
	}
	reason := byte(0)
	if cur.remaining() > 0 {
		reason, err = cur.readByte("reason code")
		if err != nil {
			return nil, err
		}
		if err := validateReason(packetType, reason); err != nil {
			return nil, err
		}
	}
	if cur.remaining() > 0 {
		if err := decodeAckProperties(cur, packetType, out); err != nil {
			return nil, err
		}
	}
	switch p := out.(type) {
	case *mqtt5.Puback:
		p.PacketID = packetID
		p.ReasonCode = reason
	case *mqtt5.Pubrec:
		p.PacketID = packetID
		p.ReasonCode = reason
	case *mqtt5.Pubrel:
		p.PacketID = packetID
		p.ReasonCode = reason
	case *mqtt5.Pubcomp:
		p.PacketID = packetID
		p.ReasonCode = reason
	}
	return out, nil
}

// decodeAckProperties 根据具体确认报文类型解码属性。
func decodeAckProperties(cur *cursor, packetType mqtt5.PacketType, out mqtt5.Packet) error {
	switch p := out.(type) {
	case *mqtt5.Puback:
		return decodeProperties(cur, packetType, p.Properties)
	case *mqtt5.Pubrec:
		return decodeProperties(cur, packetType, p.Properties)
	case *mqtt5.Pubrel:
		return decodeProperties(cur, packetType, p.Properties)
	case *mqtt5.Pubcomp:
		return decodeProperties(cur, packetType, p.Properties)
	default:
		return implementation(packetType, "packet", "unsupported ack packet")
	}
}

// decodeSubscribe 解析 SUBSCRIBE 报文中的 Packet Identifier、属性和订阅列表。
func decodeSubscribe(cur *cursor) (*mqtt5.Subscribe, error) {
	out := &mqtt5.Subscribe{
		Properties:    &mqtt5.SubscribeProperties{},
		Subscriptions: []mqtt5.SubOptions{},
	}
	var err error
	out.PacketID, err = cur.readUint16("packet identifier")
	if err != nil {
		return nil, err
	}
	if out.PacketID == 0 {
		return nil, malformed(mqtt5.SUBSCRIBE, "packet identifier", "packet identifier must be non-zero")
	}
	if err := decodeProperties(cur, mqtt5.SUBSCRIBE, out.Properties); err != nil {
		return nil, err
	}
	for cur.remaining() > 0 {
		topic, err := cur.readString("topic filter")
		if err != nil {
			return nil, err
		}
		opts, err := cur.readByte("subscription options")
		if err != nil {
			return nil, err
		}
		sub := mqtt5.SubOptions{
			Topic:             topic,
			QoS:               opts & 0x03,
			NoLocal:           opts&0x04 != 0,
			RetainAsPublished: opts&0x08 != 0,
			RetainHandling:    (opts >> 4) & 0x03,
		}
		if sub.QoS == 3 {
			return nil, malformed(mqtt5.SUBSCRIBE, "subscription options", "QoS 3 is reserved")
		}
		if sub.RetainHandling == 3 {
			return nil, malformed(mqtt5.SUBSCRIBE, "subscription options", "retain handling 3 is reserved")
		}
		if opts&0xc0 != 0 {
			return nil, malformed(mqtt5.SUBSCRIBE, "subscription options", "reserved bits must be zero")
		}
		if err := validateTopicFilter(mqtt5.SUBSCRIBE, "topic filter", topic); err != nil {
			return nil, err
		}
		out.Subscriptions = append(out.Subscriptions, sub)
	}
	if len(out.Subscriptions) == 0 {
		return nil, malformed(mqtt5.SUBSCRIBE, "topic filters", "at least one topic filter is required")
	}
	return out, nil
}

// decodeSuback 解析 SUBACK，并校验每个订阅结果原因码。
func decodeSuback(cur *cursor) (*mqtt5.Suback, error) {
	out := &mqtt5.Suback{Properties: &mqtt5.SubackProperties{}}
	var err error
	out.PacketID, err = cur.readUint16("packet identifier")
	if err != nil {
		return nil, err
	}
	if out.PacketID == 0 {
		return nil, malformed(mqtt5.SUBACK, "packet identifier", "packet identifier must be non-zero")
	}
	if err := decodeProperties(cur, mqtt5.SUBACK, out.Properties); err != nil {
		return nil, err
	}
	out.Reasons = append([]byte(nil), cur.data[cur.pos:]...)
	if len(out.Reasons) == 0 {
		return nil, malformed(mqtt5.SUBACK, "reason codes", "at least one reason code is required")
	}
	for _, reason := range out.Reasons {
		if err := validateReason(mqtt5.SUBACK, reason); err != nil {
			return nil, err
		}
	}
	cur.pos = len(cur.data)
	return out, nil
}

// decodeUnsubscribe 解析 UNSUBSCRIBE 报文中的主题过滤器列表。
func decodeUnsubscribe(cur *cursor) (*mqtt5.Unsubscribe, error) {
	out := &mqtt5.Unsubscribe{
		Properties: &mqtt5.UnsubscribeProperties{},
		Topics:     []string{},
	}
	var err error
	out.PacketID, err = cur.readUint16("packet identifier")
	if err != nil {
		return nil, err
	}
	if out.PacketID == 0 {
		return nil, malformed(mqtt5.UNSUBSCRIBE, "packet identifier", "packet identifier must be non-zero")
	}
	if err := decodeProperties(cur, mqtt5.UNSUBSCRIBE, out.Properties); err != nil {
		return nil, err
	}
	for cur.remaining() > 0 {
		topic, err := cur.readString("topic filter")
		if err != nil {
			return nil, err
		}
		if err := validateTopicFilter(mqtt5.UNSUBSCRIBE, "topic filter", topic); err != nil {
			return nil, err
		}
		out.Topics = append(out.Topics, topic)
	}
	if len(out.Topics) == 0 {
		return nil, malformed(mqtt5.UNSUBSCRIBE, "topic filters", "at least one topic filter is required")
	}
	return out, nil
}

// decodeUnsuback 解析 UNSUBACK，并校验每个取消订阅结果原因码。
func decodeUnsuback(cur *cursor) (*mqtt5.Unsuback, error) {
	out := &mqtt5.Unsuback{Properties: &mqtt5.UnsubackProperties{}}
	var err error
	out.PacketID, err = cur.readUint16("packet identifier")
	if err != nil {
		return nil, err
	}
	if out.PacketID == 0 {
		return nil, malformed(mqtt5.UNSUBACK, "packet identifier", "packet identifier must be non-zero")
	}
	if err := decodeProperties(cur, mqtt5.UNSUBACK, out.Properties); err != nil {
		return nil, err
	}
	out.Reasons = append([]byte(nil), cur.data[cur.pos:]...)
	if len(out.Reasons) == 0 {
		return nil, malformed(mqtt5.UNSUBACK, "reason codes", "at least one reason code is required")
	}
	for _, reason := range out.Reasons {
		if err := validateReason(mqtt5.UNSUBACK, reason); err != nil {
			return nil, err
		}
	}
	cur.pos = len(cur.data)
	return out, nil
}

// decodeEmpty 解析 PINGREQ/PINGRESP 这类剩余长度必须为 0 的报文。
func decodeEmpty(cur *cursor, packet mqtt5.Packet) (mqtt5.Packet, error) {
	if cur.remaining() != 0 {
		return nil, malformed(packet.PacketType(), "remaining length", "remaining length must be zero")
	}
	return packet, nil
}

// decodeDisconnect 解析 DISCONNECT，兼容剩余长度为 0 时的默认正常断开原因。
func decodeDisconnect(cur *cursor) (*mqtt5.Disconnect, error) {
	out := &mqtt5.Disconnect{Properties: &mqtt5.DisconnectProperties{}}
	if cur.remaining() == 0 {
		out.ReasonCode = mqtt5.DisconnectNormalDisconnection
		return out, nil
	}
	var err error
	out.ReasonCode, err = cur.readByte("reason code")
	if err != nil {
		return nil, err
	}
	if err := validateReason(mqtt5.DISCONNECT, out.ReasonCode); err != nil {
		return nil, err
	}
	if cur.remaining() > 0 {
		if err := decodeProperties(cur, mqtt5.DISCONNECT, out.Properties); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// decodeAuth 解析 AUTH，兼容剩余长度为 0 时的默认成功原因。
func decodeAuth(cur *cursor) (*mqtt5.Auth, error) {
	out := &mqtt5.Auth{Properties: &mqtt5.AuthProperties{}}
	if cur.remaining() == 0 {
		out.ReasonCode = mqtt5.AuthSuccess
		return out, nil
	}
	var err error
	out.ReasonCode, err = cur.readByte("reason code")
	if err != nil {
		return nil, err
	}
	if err := validateReason(mqtt5.AUTH, out.ReasonCode); err != nil {
		return nil, err
	}
	if cur.remaining() > 0 {
		if err := decodeProperties(cur, mqtt5.AUTH, out.Properties); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// encodePacket 根据具体报文结构体分派到对应编码器。
func encodePacket(packet mqtt5.Packet) ([]byte, error) {
	switch p := packet.(type) {
	case *mqtt5.Connect:
		return encodeConnect(p)
	case *mqtt5.ConnAck:
		return encodeConnack(p)
	case *mqtt5.Publish:
		return encodePublish(p)
	case *mqtt5.Puback:
		return encodeAck(mqtt5.PUBACK, p.PacketID, p.ReasonCode, p.Properties)
	case *mqtt5.Pubrec:
		return encodeAck(mqtt5.PUBREC, p.PacketID, p.ReasonCode, p.Properties)
	case *mqtt5.Pubrel:
		return encodeAck(mqtt5.PUBREL, p.PacketID, p.ReasonCode, p.Properties)
	case *mqtt5.Pubcomp:
		return encodeAck(mqtt5.PUBCOMP, p.PacketID, p.ReasonCode, p.Properties)
	case *mqtt5.Subscribe:
		return encodeSubscribe(p)
	case *mqtt5.Suback:
		return encodeCodeList(mqtt5.SUBACK, p.PacketID, p.Properties, p.Reasons)
	case *mqtt5.Unsubscribe:
		return encodeUnsubscribe(p)
	case *mqtt5.Unsuback:
		return encodeCodeList(mqtt5.UNSUBACK, p.PacketID, p.Properties, p.Reasons)
	case *mqtt5.Pingreq, *mqtt5.Pingresp:
		return nil, nil
	case *mqtt5.Disconnect:
		return encodeReasonPacket(mqtt5.DISCONNECT, p.ReasonCode, p.Properties)
	case *mqtt5.Auth:
		return encodeReasonPacket(mqtt5.AUTH, p.ReasonCode, p.Properties)
	default:
		return nil, implementation(0, "packet", "unknown packet content type")
	}
}

// encodeConnect 编码 CONNECT 的可变报头、属性和载荷字段。
func encodeConnect(p *mqtt5.Connect) ([]byte, error) {
	if err := validateConnectForEncode(p); err != nil {
		return nil, err
	}
	if p.ProtocolName != "MQTT" {
		return nil, malformed(mqtt5.CONNECT, "protocol name", "protocol name must be MQTT")
	}
	if p.ProtocolVersion != 5 {
		return nil, unsupportedVersion(p.ProtocolVersion)
	}
	flags := p.PackFlags()
	var buf bytes.Buffer
	if err := writeString(&buf, mqtt5.CONNECT, "protocol name", p.ProtocolName); err != nil {
		return nil, err
	}
	buf.WriteByte(p.ProtocolVersion)
	buf.WriteByte(flags)
	writeUint16(&buf, p.KeepAlive)
	props, err := encodeProperties(mqtt5.CONNECT, p.Properties)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(props)
	if err := writeString(&buf, mqtt5.CONNECT, "client id", p.ClientID); err != nil {
		return nil, err
	}
	if p.WillFlag {
		willProps, err := encodeProperties(mqtt5.WILLPROPERTIES, p.WillProperties)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(willProps)
		if err := validateTopicName(mqtt5.CONNECT, "will topic", p.WillTopic, false); err != nil {
			return nil, err
		}
		if err := writeString(&buf, mqtt5.CONNECT, "will topic", p.WillTopic); err != nil {
			return nil, err
		}
		if err := writeBinary(&buf, mqtt5.CONNECT, "will message", p.WillMessage); err != nil {
			return nil, err
		}
	}
	if p.UsernameFlag {
		if err := writeString(&buf, mqtt5.CONNECT, "username", p.Username); err != nil {
			return nil, err
		}
	}
	if p.PasswordFlag {
		if err := writeBinary(&buf, mqtt5.CONNECT, "password", p.Password); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func validateConnectForEncode(p *mqtt5.Connect) error {
	if p == nil {
		return malformed(mqtt5.CONNECT, "packet", "packet must not be nil")
	}
	if p.WillQOS > 2 {
		return malformed(mqtt5.CONNECT, "connect flags", "will QoS must be 0, 1, or 2")
	}
	if !p.WillFlag {
		if p.WillQOS != 0 {
			return malformed(mqtt5.CONNECT, "connect flags", "will QoS must be zero when will flag is false")
		}
		if p.WillRetain {
			return malformed(mqtt5.CONNECT, "connect flags", "will retain must be false when will flag is false")
		}
	}
	if p.PasswordFlag && !p.UsernameFlag {
		return malformed(mqtt5.CONNECT, "connect flags", "password flag requires username flag")
	}
	return nil
}

// encodeConnack 编码 CONNACK，并校验失败响应不能设置 Session Present。
func encodeConnack(p *mqtt5.ConnAck) ([]byte, error) {
	if err := validateReason(mqtt5.CONNACK, p.ReasonCode); err != nil {
		return nil, err
	}
	if p.ReasonCode != mqtt5.ConnAckSuccess && p.SessionPresent {
		return nil, malformed(mqtt5.CONNACK, "session present", "session present must be false for unsuccessful CONNACK")
	}
	var buf bytes.Buffer
	if p.SessionPresent {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteByte(p.ReasonCode)
	props, err := encodeProperties(mqtt5.CONNACK, p.Properties)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(props)
	return buf.Bytes(), nil
}

// encodePublish 编码 PUBLISH，并根据 QoS 校验 Packet Identifier 约束。
func encodePublish(p *mqtt5.Publish) ([]byte, error) {
	if p.QoS == 3 {
		return nil, malformed(mqtt5.PUBLISH, "qos", "PUBLISH QoS 3 is reserved")
	}
	if p.QoS == 0 && p.PacketID != 0 {
		return nil, malformed(mqtt5.PUBLISH, "packet identifier", "QoS 0 PUBLISH must not carry a packet identifier")
	}
	if p.QoS > 0 && p.PacketID == 0 {
		return nil, malformed(mqtt5.PUBLISH, "packet identifier", "packet identifier must be non-zero")
	}
	if p.Topic == "" {
		if p.Properties == nil || p.Properties.TopicAlias == nil {
			return nil, malformed(mqtt5.PUBLISH, "topic name", "topic name is required unless topic alias is present")
		}
	} else if err := validateTopicName(mqtt5.PUBLISH, "topic name", p.Topic, false); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := writeString(&buf, mqtt5.PUBLISH, "topic name", p.Topic); err != nil {
		return nil, err
	}
	if p.QoS > 0 {
		writeUint16(&buf, p.PacketID)
	}
	props, err := encodeProperties(mqtt5.PUBLISH, p.Properties)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(props)
	_, _ = buf.Write(p.Payload)
	return buf.Bytes(), nil
}

// encodeAck 编码 PUBACK/PUBREC/PUBREL/PUBCOMP 的共同报文布局。
func encodeAck(packetType mqtt5.PacketType, packetID uint16, reason byte, props mqtt5.Properties) ([]byte, error) {
	if packetID == 0 {
		return nil, malformed(packetType, "packet identifier", "packet identifier must be non-zero")
	}
	if err := validateReason(packetType, reason); err != nil {
		return nil, err
	}
	propBytes, err := encodeProperties(packetType, props)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writeUint16(&buf, packetID)
	// MQTT 5 允许在原因码为 0 且没有属性时省略原因码和属性长度。
	if reason == 0 && len(propBytes) == 1 {
		return buf.Bytes(), nil
	}
	buf.WriteByte(reason)
	if len(propBytes) > 1 {
		_, _ = buf.Write(propBytes)
	}
	return buf.Bytes(), nil
}

// encodeSubscribe 编码 SUBSCRIBE，并逐项校验主题过滤器和订阅选项。
func encodeSubscribe(p *mqtt5.Subscribe) ([]byte, error) {
	if p.PacketID == 0 {
		return nil, malformed(mqtt5.SUBSCRIBE, "packet identifier", "packet identifier must be non-zero")
	}
	if len(p.Subscriptions) == 0 {
		return nil, malformed(mqtt5.SUBSCRIBE, "topic filters", "at least one topic filter is required")
	}
	var buf bytes.Buffer
	writeUint16(&buf, p.PacketID)
	props, err := encodeProperties(mqtt5.SUBSCRIBE, p.Properties)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(props)
	for _, sub := range p.Subscriptions {
		if err := validateTopicFilter(mqtt5.SUBSCRIBE, "topic filter", sub.Topic); err != nil {
			return nil, err
		}
		if err := validateSubOptionsForEncode(sub); err != nil {
			return nil, err
		}
		if err := writeString(&buf, mqtt5.SUBSCRIBE, "topic filter", sub.Topic); err != nil {
			return nil, err
		}
		buf.WriteByte(sub.Pack())
	}
	return buf.Bytes(), nil
}

func validateSubOptionsForEncode(sub mqtt5.SubOptions) error {
	if sub.QoS > 2 {
		return malformed(mqtt5.SUBSCRIBE, "subscription options", "QoS must be between 0 and 2")
	}
	if sub.RetainHandling > 2 {
		return malformed(mqtt5.SUBSCRIBE, "subscription options", "retain handling must be between 0 and 2")
	}
	return nil
}

// encodeCodeList 编码 SUBACK/UNSUBACK 中的原因码列表。
func encodeCodeList(packetType mqtt5.PacketType, packetID uint16, props mqtt5.Properties, reasons []byte) ([]byte, error) {
	if packetID == 0 {
		return nil, malformed(packetType, "packet identifier", "packet identifier must be non-zero")
	}
	if len(reasons) == 0 {
		return nil, malformed(packetType, "reason codes", "at least one reason code is required")
	}
	for _, reason := range reasons {
		if err := validateReason(packetType, reason); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	writeUint16(&buf, packetID)
	propBytes, err := encodeProperties(packetType, props)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(propBytes)
	_, _ = buf.Write(reasons)
	return buf.Bytes(), nil
}

// encodeUnsubscribe 编码 UNSUBSCRIBE，并校验至少包含一个主题过滤器。
func encodeUnsubscribe(p *mqtt5.Unsubscribe) ([]byte, error) {
	if p.PacketID == 0 {
		return nil, malformed(mqtt5.UNSUBSCRIBE, "packet identifier", "packet identifier must be non-zero")
	}
	if len(p.Topics) == 0 {
		return nil, malformed(mqtt5.UNSUBSCRIBE, "topic filters", "at least one topic filter is required")
	}
	var buf bytes.Buffer
	writeUint16(&buf, p.PacketID)
	props, err := encodeProperties(mqtt5.UNSUBSCRIBE, p.Properties)
	if err != nil {
		return nil, err
	}
	_, _ = buf.Write(props)
	for _, topic := range p.Topics {
		if err := validateTopicFilter(mqtt5.UNSUBSCRIBE, "topic filter", topic); err != nil {
			return nil, err
		}
		if err := writeString(&buf, mqtt5.UNSUBSCRIBE, "topic filter", topic); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// encodeReasonPacket 编码 DISCONNECT/AUTH 这类“原因码 + 属性”的报文。
func encodeReasonPacket(packetType mqtt5.PacketType, reason byte, props mqtt5.Properties) ([]byte, error) {
	if err := validateReason(packetType, reason); err != nil {
		return nil, err
	}
	propBytes, err := encodeProperties(packetType, props)
	if err != nil {
		return nil, err
	}
	// 默认成功/正常原因且没有属性时，MQTT 5 允许整个可变报头省略。
	if reason == 0 && len(propBytes) == 1 {
		return nil, nil
	}
	var buf bytes.Buffer
	buf.WriteByte(reason)
	if len(propBytes) > 1 {
		_, _ = buf.Write(propBytes)
	}
	return buf.Bytes(), nil
}

// validateTopicName 校验发布主题名；主题名不能包含通配符。
func validateTopicName(packet mqtt5.PacketType, field, topic string, allowEmpty bool) error {
	if topic == "" {
		if allowEmpty {
			return nil
		}
		return malformed(packet, field, "topic name must be non-empty")
	}
	if strings.ContainsAny(topic, "+#") {
		return malformed(packet, field, "topic name must not contain wildcards")
	}
	return nil
}

// validateTopicFilter 校验订阅主题过滤器的通配符和共享订阅格式。
func validateTopicFilter(packet mqtt5.PacketType, field, topic string) error {
	if err := topicutil.ValidateTopicFilterSyntax(topic); err != nil {
		return malformed(packet, field, err.Error())
	}
	return nil
}
