package wire

import (
	"bytes"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// PropertyValueType 描述属性在线上编码时使用的值类型。
type PropertyValueType int

// RepeatPolicy 描述同一个属性是否允许在一个属性段内重复出现。
type RepeatPolicy int

// PacketMask 用位图表示某个属性允许出现在哪些报文类型中。
type PacketMask uint32

const (
	// 属性值类型与 MQTT 5 规范中的属性编码格式一一对应。
	propertyByte PropertyValueType = iota + 1
	propertyTwoByteInteger
	propertyFourByteInteger
	propertyVariableByteInteger
	propertyBinaryData
	propertyString
	propertyStringPair
)

const (
	// repeatNever 表示属性在同一报文中最多出现一次。
	repeatNever RepeatPolicy = iota
	// repeatAllowed 表示属性可以重复出现，例如 User Property。
	repeatAllowed
)

// PropertySpec 描述一个 MQTT 5 属性的标识符、类型、适用报文和重复策略。
type PropertySpec struct {
	// ID 是属性在字节流中的属性标识符。
	ID byte
	// Name 是用于错误信息和调试输出的规范名称。
	Name string
	// Type 是属性值的编码类型。
	Type PropertyValueType
	// Allowed 是允许携带该属性的报文类型位图。
	Allowed PacketMask
	// Repeat 描述属性是否可以重复出现。
	Repeat RepeatPolicy
}

// propertySpecs 是属性编解码和合法性校验的统一规格表。
var propertySpecs = map[byte]PropertySpec{
	mqtt5.PropPayloadFormat: {
		ID:      mqtt5.PropPayloadFormat,
		Name:    "Payload Format Indicator",
		Type:    propertyByte,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropMessageExpiry: {
		ID:      mqtt5.PropMessageExpiry,
		Name:    "Message Expiry Interval",
		Type:    propertyFourByteInteger,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropContentType: {
		ID:      mqtt5.PropContentType,
		Name:    "Content Type",
		Type:    propertyString,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropResponseTopic: {
		ID:      mqtt5.PropResponseTopic,
		Name:    "Response Topic",
		Type:    propertyString,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropCorrelationData: {
		ID:      mqtt5.PropCorrelationData,
		Name:    "Correlation Data",
		Type:    propertyBinaryData,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropSubscriptionIdentifier: {
		ID:      mqtt5.PropSubscriptionIdentifier,
		Name:    "Subscription Identifier",
		Type:    propertyVariableByteInteger,
		Allowed: mask(mqtt5.PUBLISH, mqtt5.SUBSCRIBE),
	},
	mqtt5.PropSessionExpiryInterval: {
		ID:      mqtt5.PropSessionExpiryInterval,
		Name:    "Session Expiry Interval",
		Type:    propertyFourByteInteger,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK, mqtt5.DISCONNECT),
	},
	mqtt5.PropAssignedClientID: {
		ID:      mqtt5.PropAssignedClientID,
		Name:    "Assigned Client Identifier",
		Type:    propertyString,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropServerKeepAlive: {
		ID:      mqtt5.PropServerKeepAlive,
		Name:    "Server Keep Alive",
		Type:    propertyTwoByteInteger,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropAuthMethod: {
		ID:      mqtt5.PropAuthMethod,
		Name:    "Authentication Method",
		Type:    propertyString,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK, mqtt5.AUTH),
	},
	mqtt5.PropAuthData: {
		ID:      mqtt5.PropAuthData,
		Name:    "Authentication Data",
		Type:    propertyBinaryData,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK, mqtt5.AUTH),
	},
	mqtt5.PropRequestProblemInfo: {
		ID:      mqtt5.PropRequestProblemInfo,
		Name:    "Request Problem Information",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNECT),
	},
	mqtt5.PropWillDelayInterval: {
		ID:      mqtt5.PropWillDelayInterval,
		Name:    "Will Delay Interval",
		Type:    propertyFourByteInteger,
		Allowed: mask(mqtt5.WILLPROPERTIES),
	},
	mqtt5.PropRequestResponseInfo: {
		ID:      mqtt5.PropRequestResponseInfo,
		Name:    "Request Response Information",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNECT),
	},
	mqtt5.PropResponseInfo: {
		ID:      mqtt5.PropResponseInfo,
		Name:    "Response Information",
		Type:    propertyString,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropServerReference: {
		ID:      mqtt5.PropServerReference,
		Name:    "Server Reference",
		Type:    propertyString,
		Allowed: mask(mqtt5.CONNACK, mqtt5.DISCONNECT),
	},
	mqtt5.PropReasonString: {
		ID:      mqtt5.PropReasonString,
		Name:    "Reason String",
		Type:    propertyString,
		Allowed: mask(mqtt5.CONNACK, mqtt5.PUBACK, mqtt5.PUBREC, mqtt5.PUBREL, mqtt5.PUBCOMP, mqtt5.SUBACK, mqtt5.UNSUBACK, mqtt5.DISCONNECT, mqtt5.AUTH),
	},
	mqtt5.PropReceiveMaximum: {
		ID:      mqtt5.PropReceiveMaximum,
		Name:    "Receive Maximum",
		Type:    propertyTwoByteInteger,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK),
	},
	mqtt5.PropTopicAliasMaximum: {
		ID:      mqtt5.PropTopicAliasMaximum,
		Name:    "Topic Alias Maximum",
		Type:    propertyTwoByteInteger,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK),
	},
	mqtt5.PropTopicAlias: {
		ID:      mqtt5.PropTopicAlias,
		Name:    "Topic Alias",
		Type:    propertyTwoByteInteger,
		Allowed: mask(mqtt5.PUBLISH),
	},
	mqtt5.PropMaximumQOS: {
		ID:      mqtt5.PropMaximumQOS,
		Name:    "Maximum QoS",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropRetainAvailable: {
		ID:      mqtt5.PropRetainAvailable,
		Name:    "Retain Available",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropUser: {
		ID:      mqtt5.PropUser,
		Name:    "User Property",
		Type:    propertyStringPair,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK, mqtt5.PUBLISH, mqtt5.WILLPROPERTIES, mqtt5.PUBACK, mqtt5.PUBREC, mqtt5.PUBREL, mqtt5.PUBCOMP, mqtt5.SUBSCRIBE, mqtt5.SUBACK, mqtt5.UNSUBSCRIBE, mqtt5.UNSUBACK, mqtt5.DISCONNECT, mqtt5.AUTH),
		Repeat:  repeatAllowed,
	},
	mqtt5.PropMaximumPacketSize: {
		ID:      mqtt5.PropMaximumPacketSize,
		Name:    "Maximum Packet Size",
		Type:    propertyFourByteInteger,
		Allowed: mask(mqtt5.CONNECT, mqtt5.CONNACK),
	},
	mqtt5.PropWildcardSubAvailable: {
		ID:      mqtt5.PropWildcardSubAvailable,
		Name:    "Wildcard Subscription Available",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropSubIDAvailable: {
		ID:      mqtt5.PropSubIDAvailable,
		Name:    "Subscription Identifier Available",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNACK),
	},
	mqtt5.PropSharedSubAvailable: {
		ID:      mqtt5.PropSharedSubAvailable,
		Name:    "Shared Subscription Available",
		Type:    propertyByte,
		Allowed: mask(mqtt5.CONNACK),
	},
}

// propertyValue 是解码后的属性值联合体，使用字段由 PropertySpec.Type 决定。
type propertyValue struct {
	byteValue   byte
	uint16Value uint16
	uint32Value uint32
	vbiValue    int
	binaryValue []byte
	stringValue string
	userValue   mqtt5.User
}

// mask 将多个报文类型转换成属性允许出现的位图。
func mask(packetTypes ...mqtt5.PacketType) PacketMask {
	var out PacketMask
	for _, packetType := range packetTypes {
		out |= 1 << packetType
	}
	return out
}

// packetAllowed 判断某个属性规格是否允许出现在指定报文中。
func packetAllowed(spec PropertySpec, packet mqtt5.PacketType) bool {
	return spec.Allowed&(1<<packet) != 0
}

// decodeProperties 解码 MQTT 属性长度字段和属性列表，并写入目标属性结构体。
func decodeProperties(cur *cursor, packet mqtt5.PacketType, props mqtt5.Properties) error {
	propertyLength, err := cur.readVBI("property length")
	if err != nil {
		return err
	}
	if propertyLength > cur.remaining() {
		return malformed(packet, "property length", "property length exceeds remaining packet body")
	}
	propCur := newCursor(packet, cur.data[cur.pos:cur.pos+propertyLength])
	cur.pos += propertyLength

	seen := map[byte]struct{}{}
	for propCur.remaining() > 0 {
		id, err := propCur.readByte("property id")
		if err != nil {
			return err
		}
		spec, ok := propertySpecs[id]
		if !ok {
			return malformed(packet, "property id", "unknown property")
		}
		if !packetAllowed(spec, packet) {
			return malformed(packet, spec.Name, "property is not allowed for packet")
		}
		if !propertyRepeatAllowed(spec, packet) {
			if _, ok := seen[id]; ok {
				return malformed(packet, spec.Name, "duplicate singleton property")
			}
			seen[id] = struct{}{}
		}

		value, err := propCur.readPropertyValue(spec)
		if err != nil {
			return err
		}
		if err := validatePropertyValue(packet, spec, value); err != nil {
			return err
		}
		if err := assignProperty(packet, props, spec, value); err != nil {
			return err
		}
	}
	return nil
}

func propertyRepeatAllowed(spec PropertySpec, packet mqtt5.PacketType) bool {
	if spec.Repeat == repeatAllowed {
		return true
	}
	// MQTT5: Subscription Identifier can repeat in PUBLISH, but must be single in SUBSCRIBE.
	return spec.ID == mqtt5.PropSubscriptionIdentifier && packet == mqtt5.PUBLISH
}

// readPropertyValue 按属性规格读取对应类型的属性值。
func (c *cursor) readPropertyValue(spec PropertySpec) (propertyValue, error) {
	switch spec.Type {
	case propertyByte:
		value, err := c.readByte(spec.Name)
		return propertyValue{byteValue: value}, err
	case propertyTwoByteInteger:
		value, err := c.readUint16(spec.Name)
		return propertyValue{uint16Value: value}, err
	case propertyFourByteInteger:
		value, err := c.readUint32(spec.Name)
		return propertyValue{uint32Value: value}, err
	case propertyVariableByteInteger:
		value, err := c.readVBI(spec.Name)
		return propertyValue{vbiValue: value}, err
	case propertyBinaryData:
		value, err := c.readBinary(spec.Name)
		return propertyValue{binaryValue: value}, err
	case propertyString:
		value, err := c.readString(spec.Name)
		return propertyValue{stringValue: value}, err
	case propertyStringPair:
		key, err := c.readString(spec.Name + " key")
		if err != nil {
			return propertyValue{}, err
		}
		value, err := c.readString(spec.Name + " value")
		if err != nil {
			return propertyValue{}, err
		}
		return propertyValue{userValue: mqtt5.User{Key: key, Value: value}}, nil
	default:
		return propertyValue{}, implementation(c.packet, spec.Name, "unknown property value type")
	}
}

// validatePropertyValue 校验属性值是否满足 MQTT 5 对布尔、非零和主题名的约束。
func validatePropertyValue(packet mqtt5.PacketType, spec PropertySpec, value propertyValue) error {
	switch spec.ID {
	case mqtt5.PropPayloadFormat,
		mqtt5.PropRequestProblemInfo,
		mqtt5.PropRequestResponseInfo,
		mqtt5.PropRetainAvailable,
		mqtt5.PropWildcardSubAvailable,
		mqtt5.PropSubIDAvailable,
		mqtt5.PropSharedSubAvailable:
		if value.byteValue > 1 {
			return malformed(packet, spec.Name, "boolean byte property must be 0 or 1")
		}
	case mqtt5.PropMaximumQOS:
		if value.byteValue > 1 {
			return malformed(packet, spec.Name, "maximum QoS must be 0 or 1")
		}
	case mqtt5.PropReceiveMaximum:
		if value.uint16Value == 0 {
			return malformed(packet, spec.Name, "receive maximum must be non-zero")
		}
	case mqtt5.PropTopicAlias:
		if value.uint16Value == 0 {
			return malformed(packet, spec.Name, "topic alias must be non-zero")
		}
	case mqtt5.PropMaximumPacketSize:
		if value.uint32Value == 0 {
			return malformed(packet, spec.Name, "maximum packet size must be non-zero")
		}
	case mqtt5.PropSubscriptionIdentifier:
		if value.vbiValue == 0 {
			return malformed(packet, spec.Name, "subscription identifier must be non-zero")
		}
	case mqtt5.PropResponseTopic:
		if err := validateTopicName(packet, spec.Name, value.stringValue, false); err != nil {
			return err
		}
	}
	return nil
}

// assignProperty 将解码后的属性值分派写入对应报文的属性结构体。
func assignProperty(packet mqtt5.PacketType, props mqtt5.Properties, spec PropertySpec, value propertyValue) error {
	if props == nil {
		return nil
	}
	switch p := props.(type) {
	case *mqtt5.PublishProperties:
		return assignPublishProperty(p, spec.ID, value)
	case *mqtt5.ConnectProperties:
		return assignConnectProperty(p, spec.ID, value)
	case *mqtt5.ConnAckProperties:
		return assignConnackProperty(p, spec.ID, value)
	case *mqtt5.SubscribeProperties:
		return assignSubscribeProperty(p, spec.ID, value)
	case *mqtt5.SubackProperties:
		return assignSubackProperty(p, spec.ID, value)
	case *mqtt5.UnsubscribeProperties:
		return assignUnsubscribeProperty(p, spec.ID, value)
	case *mqtt5.UnsubackProperties:
		return assignUnsubackProperty(p, spec.ID, value)
	case *mqtt5.PubackProperties:
		return assignPubackProperty(p, spec.ID, value)
	case *mqtt5.PubrecProperties:
		return assignPubrecProperty(p, spec.ID, value)
	case *mqtt5.PubrelProperties:
		return assignPubrelProperty(p, spec.ID, value)
	case *mqtt5.PubcompProperties:
		return assignPubcompProperty(p, spec.ID, value)
	case *mqtt5.DisconnectProperties:
		return assignDisconnectProperty(p, spec.ID, value)
	case *mqtt5.AuthProperties:
		return assignAuthProperty(p, spec.ID, value)
	default:
		return implementation(packet, spec.Name, "unsupported properties struct")
	}
}

// assignPublishProperty 写入 PUBLISH/Will Properties 支持的属性。
func assignPublishProperty(p *mqtt5.PublishProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropPayloadFormat:
		p.PayloadFormat = &value.byteValue
	case mqtt5.PropMessageExpiry:
		p.MessageExpiry = &value.uint32Value
	case mqtt5.PropWillDelayInterval:
		p.WillDelayInterval = &value.uint32Value
	case mqtt5.PropContentType:
		p.ContentType = value.stringValue
	case mqtt5.PropResponseTopic:
		p.ResponseTopic = value.stringValue
	case mqtt5.PropCorrelationData:
		p.CorrelationData = value.binaryValue
	case mqtt5.PropTopicAlias:
		p.TopicAlias = &value.uint16Value
	case mqtt5.PropSubscriptionIdentifier:
		p.SubscriptionIdentifier = append(p.SubscriptionIdentifier, value.vbiValue)
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

// assignConnectProperty 写入 CONNECT 支持的属性。
func assignConnectProperty(p *mqtt5.ConnectProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropSessionExpiryInterval:
		p.SessionExpiryInterval = &value.uint32Value
	case mqtt5.PropAuthMethod:
		p.AuthMethod = value.stringValue
	case mqtt5.PropAuthData:
		p.AuthData = value.binaryValue
	case mqtt5.PropRequestProblemInfo:
		p.RequestProblemInfo = &value.byteValue
	case mqtt5.PropRequestResponseInfo:
		p.RequestResponseInfo = &value.byteValue
	case mqtt5.PropReceiveMaximum:
		p.ReceiveMaximum = &value.uint16Value
	case mqtt5.PropTopicAliasMaximum:
		p.TopicAliasMaximum = &value.uint16Value
	case mqtt5.PropMaximumPacketSize:
		p.MaximumPacketSize = &value.uint32Value
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

// assignConnackProperty 写入 CONNACK 支持的属性。
func assignConnackProperty(p *mqtt5.ConnAckProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropSessionExpiryInterval:
		p.SessionExpiryInterval = &value.uint32Value
	case mqtt5.PropAssignedClientID:
		p.AssignedClientID = value.stringValue
	case mqtt5.PropServerKeepAlive:
		p.ServerKeepAlive = &value.uint16Value
	case mqtt5.PropAuthMethod:
		p.AuthMethod = value.stringValue
	case mqtt5.PropAuthData:
		p.AuthData = value.binaryValue
	case mqtt5.PropResponseInfo:
		p.ResponseInfo = value.stringValue
	case mqtt5.PropServerReference:
		p.ServerReference = value.stringValue
	case mqtt5.PropReasonString:
		p.ReasonString = value.stringValue
	case mqtt5.PropReceiveMaximum:
		p.ReceiveMaximum = &value.uint16Value
	case mqtt5.PropTopicAliasMaximum:
		p.TopicAliasMaximum = &value.uint16Value
	case mqtt5.PropMaximumQOS:
		p.MaximumQOS = &value.byteValue
	case mqtt5.PropRetainAvailable:
		p.RetainAvailable = &value.byteValue
	case mqtt5.PropWildcardSubAvailable:
		p.WildcardSubAvailable = &value.byteValue
	case mqtt5.PropSubIDAvailable:
		p.SubIDAvailable = &value.byteValue
	case mqtt5.PropSharedSubAvailable:
		p.SharedSubAvailable = &value.byteValue
	case mqtt5.PropMaximumPacketSize:
		p.MaximumPacketSize = &value.uint32Value
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

// assignSubscribeProperty 写入 SUBSCRIBE 支持的属性。
func assignSubscribeProperty(p *mqtt5.SubscribeProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropSubscriptionIdentifier:
		p.SubscriptionIdentifier = &value.vbiValue
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

// assignReasonUserProperty 写入只包含 Reason String 和 User Property 的属性结构体。
func assignReasonUserProperty(reason *string, users *[]mqtt5.User, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropReasonString:
		*reason = value.stringValue
	case mqtt5.PropUser:
		*users = append(*users, value.userValue)
	}
	return nil
}

func assignSubackProperty(p *mqtt5.SubackProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignUnsubscribeProperty(p *mqtt5.UnsubscribeProperties, id byte, value propertyValue) error {
	if id == mqtt5.PropUser {
		p.User = append(p.User, value.userValue)
	}
	return nil
}

func assignUnsubackProperty(p *mqtt5.UnsubackProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignPubackProperty(p *mqtt5.PubackProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignPubrecProperty(p *mqtt5.PubrecProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignPubrelProperty(p *mqtt5.PubrelProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignPubcompProperty(p *mqtt5.PubcompProperties, id byte, value propertyValue) error {
	return assignReasonUserProperty(&p.ReasonString, &p.User, id, value)
}

func assignDisconnectProperty(p *mqtt5.DisconnectProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropSessionExpiryInterval:
		p.SessionExpiryInterval = &value.uint32Value
	case mqtt5.PropServerReference:
		p.ServerReference = value.stringValue
	case mqtt5.PropReasonString:
		p.ReasonString = value.stringValue
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

func assignAuthProperty(p *mqtt5.AuthProperties, id byte, value propertyValue) error {
	switch id {
	case mqtt5.PropAuthMethod:
		p.AuthMethod = value.stringValue
	case mqtt5.PropAuthData:
		p.AuthData = value.binaryValue
	case mqtt5.PropReasonString:
		p.ReasonString = value.stringValue
	case mqtt5.PropUser:
		p.User = append(p.User, value.userValue)
	}
	return nil
}

// encodeProperties 编码属性列表，并在前面补上属性总长度 VBI。
func encodeProperties(packet mqtt5.PacketType, props mqtt5.Properties) ([]byte, error) {
	var body bytes.Buffer
	if props != nil {
		if err := appendProperties(&body, packet, props); err != nil {
			return nil, err
		}
	}
	var out bytes.Buffer
	if err := writeVBI(&out, packet, "property length", body.Len()); err != nil {
		return nil, err
	}
	_, _ = out.Write(body.Bytes())
	return out.Bytes(), nil
}

// appendProperties 根据属性结构体类型追加具体属性字段。
func appendProperties(buf *bytes.Buffer, packet mqtt5.PacketType, props mqtt5.Properties) error {
	switch p := props.(type) {
	case *mqtt5.PublishProperties:
		return appendPublishProperties(buf, packet, p)
	case *mqtt5.ConnectProperties:
		return appendConnectProperties(buf, packet, p)
	case *mqtt5.ConnAckProperties:
		return appendConnackProperties(buf, packet, p)
	case *mqtt5.SubscribeProperties:
		return appendSubscribeProperties(buf, packet, p)
	case *mqtt5.SubackProperties:
		return appendSubackProperties(buf, packet, p)
	case *mqtt5.UnsubscribeProperties:
		return appendUnsubscribeProperties(buf, packet, p)
	case *mqtt5.UnsubackProperties:
		return appendUnsubackProperties(buf, packet, p)
	case *mqtt5.PubackProperties:
		return appendPubackProperties(buf, packet, p)
	case *mqtt5.PubrecProperties:
		return appendPubrecProperties(buf, packet, p)
	case *mqtt5.PubrelProperties:
		return appendPubrelProperties(buf, packet, p)
	case *mqtt5.PubcompProperties:
		return appendPubcompProperties(buf, packet, p)
	case *mqtt5.DisconnectProperties:
		return appendDisconnectProperties(buf, packet, p)
	case *mqtt5.AuthProperties:
		return appendAuthProperties(buf, packet, p)
	default:
		return nil
	}
}

func appendSubackProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.SubackProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendUnsubscribeProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.UnsubscribeProperties) error {
	if p == nil {
		return nil
	}
	return appendUserProperties(buf, packet, p.User)
}

func appendUnsubackProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.UnsubackProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendPubackProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.PubackProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendPubrecProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.PubrecProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendPubrelProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.PubrelProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendPubcompProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.PubcompProperties) error {
	if p == nil {
		return nil
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendDisconnectProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.DisconnectProperties) error {
	if p == nil {
		return nil
	}
	if p.SessionExpiryInterval != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropSessionExpiryInterval, *p.SessionExpiryInterval); err != nil {
			return err
		}
	}
	if p.ServerReference != "" {
		if err := appendPropString(buf, packet, mqtt5.PropServerReference, p.ServerReference); err != nil {
			return err
		}
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

func appendAuthProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.AuthProperties) error {
	if p == nil {
		return nil
	}
	if p.AuthMethod != "" {
		if err := appendPropString(buf, packet, mqtt5.PropAuthMethod, p.AuthMethod); err != nil {
			return err
		}
	}
	if p.AuthData != nil {
		if err := appendPropBinary(buf, packet, mqtt5.PropAuthData, p.AuthData); err != nil {
			return err
		}
	}
	return appendReasonUserProperties(buf, packet, p.ReasonString, p.User)
}

// appendPublishProperties 追加 PUBLISH/Will Properties 支持的属性字段。
func appendPublishProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.PublishProperties) error {
	if p == nil {
		return nil
	}
	if p.PayloadFormat != nil {
		if err := appendPropByte(buf, packet, mqtt5.PropPayloadFormat, *p.PayloadFormat); err != nil {
			return err
		}
	}
	if p.MessageExpiry != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropMessageExpiry, *p.MessageExpiry); err != nil {
			return err
		}
	}
	if p.WillDelayInterval != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropWillDelayInterval, *p.WillDelayInterval); err != nil {
			return err
		}
	}
	if p.ContentType != "" {
		if err := appendPropString(buf, packet, mqtt5.PropContentType, p.ContentType); err != nil {
			return err
		}
	}
	if p.ResponseTopic != "" {
		if err := appendPropString(buf, packet, mqtt5.PropResponseTopic, p.ResponseTopic); err != nil {
			return err
		}
	}
	if p.CorrelationData != nil {
		if err := appendPropBinary(buf, packet, mqtt5.PropCorrelationData, p.CorrelationData); err != nil {
			return err
		}
	}
	if p.TopicAlias != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropTopicAlias, *p.TopicAlias); err != nil {
			return err
		}
	}
	for _, id := range p.SubscriptionIdentifier {
		if err := appendPropVBI(buf, packet, mqtt5.PropSubscriptionIdentifier, id); err != nil {
			return err
		}
	}
	return appendUserProperties(buf, packet, p.User)
}

// appendConnectProperties 追加 CONNECT 属性字段。
func appendConnectProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.ConnectProperties) error {
	if p == nil {
		return nil
	}
	if p.SessionExpiryInterval != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropSessionExpiryInterval, *p.SessionExpiryInterval); err != nil {
			return err
		}
	}
	if p.AuthMethod != "" {
		if err := appendPropString(buf, packet, mqtt5.PropAuthMethod, p.AuthMethod); err != nil {
			return err
		}
	}
	if p.AuthData != nil {
		if err := appendPropBinary(buf, packet, mqtt5.PropAuthData, p.AuthData); err != nil {
			return err
		}
	}
	if p.RequestProblemInfo != nil {
		if err := appendPropByte(buf, packet, mqtt5.PropRequestProblemInfo, *p.RequestProblemInfo); err != nil {
			return err
		}
	}
	if p.RequestResponseInfo != nil {
		if err := appendPropByte(buf, packet, mqtt5.PropRequestResponseInfo, *p.RequestResponseInfo); err != nil {
			return err
		}
	}
	if p.ReceiveMaximum != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropReceiveMaximum, *p.ReceiveMaximum); err != nil {
			return err
		}
	}
	if p.TopicAliasMaximum != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropTopicAliasMaximum, *p.TopicAliasMaximum); err != nil {
			return err
		}
	}
	if p.MaximumPacketSize != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropMaximumPacketSize, *p.MaximumPacketSize); err != nil {
			return err
		}
	}
	return appendUserProperties(buf, packet, p.User)
}

// appendConnackProperties 追加 CONNACK 属性字段。
func appendConnackProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.ConnAckProperties) error {
	if p == nil {
		return nil
	}
	if p.SessionExpiryInterval != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropSessionExpiryInterval, *p.SessionExpiryInterval); err != nil {
			return err
		}
	}
	stringProps := []struct {
		id    byte
		value string
	}{
		{mqtt5.PropAssignedClientID, p.AssignedClientID},
		{mqtt5.PropAuthMethod, p.AuthMethod},
		{mqtt5.PropResponseInfo, p.ResponseInfo},
		{mqtt5.PropServerReference, p.ServerReference},
		{mqtt5.PropReasonString, p.ReasonString},
	}
	for _, prop := range stringProps {
		if prop.value != "" {
			if err := appendPropString(buf, packet, prop.id, prop.value); err != nil {
				return err
			}
		}
	}
	if p.ServerKeepAlive != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropServerKeepAlive, *p.ServerKeepAlive); err != nil {
			return err
		}
	}
	if p.AuthData != nil {
		if err := appendPropBinary(buf, packet, mqtt5.PropAuthData, p.AuthData); err != nil {
			return err
		}
	}
	if p.ReceiveMaximum != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropReceiveMaximum, *p.ReceiveMaximum); err != nil {
			return err
		}
	}
	if p.TopicAliasMaximum != nil {
		if err := appendPropUint16(buf, packet, mqtt5.PropTopicAliasMaximum, *p.TopicAliasMaximum); err != nil {
			return err
		}
	}
	byteProps := []struct {
		id    byte
		value *byte
	}{
		{mqtt5.PropMaximumQOS, p.MaximumQOS},
		{mqtt5.PropRetainAvailable, p.RetainAvailable},
		{mqtt5.PropWildcardSubAvailable, p.WildcardSubAvailable},
		{mqtt5.PropSubIDAvailable, p.SubIDAvailable},
		{mqtt5.PropSharedSubAvailable, p.SharedSubAvailable},
	}
	for _, prop := range byteProps {
		if prop.value != nil {
			if err := appendPropByte(buf, packet, prop.id, *prop.value); err != nil {
				return err
			}
		}
	}
	if p.MaximumPacketSize != nil {
		if err := appendPropUint32(buf, packet, mqtt5.PropMaximumPacketSize, *p.MaximumPacketSize); err != nil {
			return err
		}
	}
	return appendUserProperties(buf, packet, p.User)
}

// appendSubscribeProperties 追加 SUBSCRIBE 属性字段。
func appendSubscribeProperties(buf *bytes.Buffer, packet mqtt5.PacketType, p *mqtt5.SubscribeProperties) error {
	if p == nil {
		return nil
	}
	if p.SubscriptionIdentifier != nil {
		if err := appendPropVBI(buf, packet, mqtt5.PropSubscriptionIdentifier, *p.SubscriptionIdentifier); err != nil {
			return err
		}
	}
	return appendUserProperties(buf, packet, p.User)
}

// appendReasonUserProperties 追加 Reason String 和 User Property。
func appendReasonUserProperties(buf *bytes.Buffer, packet mqtt5.PacketType, reason string, users []mqtt5.User) error {
	if reason != "" {
		if err := appendPropString(buf, packet, mqtt5.PropReasonString, reason); err != nil {
			return err
		}
	}
	return appendUserProperties(buf, packet, users)
}

// appendUserProperties 按 MQTT String Pair 格式追加所有 User Property。
func appendUserProperties(buf *bytes.Buffer, packet mqtt5.PacketType, users []mqtt5.User) error {
	for _, user := range users {
		if err := appendPropertyHeader(buf, packet, mqtt5.PropUser); err != nil {
			return err
		}
		if err := writeString(buf, packet, "User Property key", user.Key); err != nil {
			return err
		}
		if err := writeString(buf, packet, "User Property value", user.Value); err != nil {
			return err
		}
	}
	return nil
}

// appendPropByte 追加单字节属性并执行取值校验。
func appendPropByte(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value byte) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	if err := validatePropertyValue(packet, propertySpecs[id], propertyValue{byteValue: value}); err != nil {
		return err
	}
	buf.WriteByte(value)
	return nil
}

// appendPropUint16 追加二字节整数属性并执行取值校验。
func appendPropUint16(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value uint16) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	if err := validatePropertyValue(packet, propertySpecs[id], propertyValue{uint16Value: value}); err != nil {
		return err
	}
	writeUint16(buf, value)
	return nil
}

// appendPropUint32 追加四字节整数属性并执行取值校验。
func appendPropUint32(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value uint32) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	if err := validatePropertyValue(packet, propertySpecs[id], propertyValue{uint32Value: value}); err != nil {
		return err
	}
	writeUint32(buf, value)
	return nil
}

// appendPropVBI 追加 Variable Byte Integer 属性并执行取值校验。
func appendPropVBI(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value int) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	if err := validatePropertyValue(packet, propertySpecs[id], propertyValue{vbiValue: value}); err != nil {
		return err
	}
	return writeVBI(buf, packet, propertySpecs[id].Name, value)
}

// appendPropBinary 追加二进制数据属性。
func appendPropBinary(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value []byte) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	return writeBinary(buf, packet, propertySpecs[id].Name, value)
}

// appendPropString 追加 UTF-8 字符串属性并执行取值校验。
func appendPropString(buf *bytes.Buffer, packet mqtt5.PacketType, id byte, value string) error {
	if err := appendPropertyHeader(buf, packet, id); err != nil {
		return err
	}
	if err := validatePropertyValue(packet, propertySpecs[id], propertyValue{stringValue: value}); err != nil {
		return err
	}
	return writeString(buf, packet, propertySpecs[id].Name, value)
}

// appendPropertyHeader 校验属性 ID 和适用报文后写入属性头。
func appendPropertyHeader(buf *bytes.Buffer, packet mqtt5.PacketType, id byte) error {
	spec, ok := propertySpecs[id]
	if !ok {
		return malformed(packet, "property id", "unknown property")
	}
	if !packetAllowed(spec, packet) {
		return malformed(packet, spec.Name, "property is not allowed for packet")
	}
	buf.WriteByte(id)
	return nil
}
