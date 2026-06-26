package mqtt5

import (
	"fmt"
	"strings"
)

const (
	// 以下常量是 MQTT 5 规范定义的属性标识符，编码时作为属性头的第一个字节。
	PropPayloadFormat          byte = 1
	PropMessageExpiry          byte = 2
	PropContentType            byte = 3
	PropResponseTopic          byte = 8
	PropCorrelationData        byte = 9
	PropSubscriptionIdentifier byte = 11
	PropSessionExpiryInterval  byte = 17
	PropAssignedClientID       byte = 18
	PropServerKeepAlive        byte = 19
	PropAuthMethod             byte = 21
	PropAuthData               byte = 22
	PropRequestProblemInfo     byte = 23
	PropWillDelayInterval      byte = 24
	PropRequestResponseInfo    byte = 25
	PropResponseInfo           byte = 26
	PropServerReference        byte = 28
	PropReasonString           byte = 31
	PropReceiveMaximum         byte = 33
	PropTopicAliasMaximum      byte = 34
	PropTopicAlias             byte = 35
	PropMaximumQOS             byte = 36
	PropRetainAvailable        byte = 37
	PropUser                   byte = 38
	PropMaximumPacketSize      byte = 39
	PropWildcardSubAvailable   byte = 40
	PropSubIDAvailable         byte = 41
	PropSharedSubAvailable     byte = 42
)

// User 表示 MQTT 5 User Property 中的一组键值对。
type User struct {
	Key   string
	Value string
}

// Properties 是所有属性结构体共享的最小能力。
type Properties interface {
	// GetUserProperties 返回报文携带的 User Property 列表。
	GetUserProperties() []User
	// SetUserProperties 覆盖报文携带的 User Property 列表。
	SetUserProperties([]User)
	fmt.Stringer
}

// PublishProperties 保存 PUBLISH 报文和遗嘱消息可使用的属性。
type PublishProperties struct {
	PayloadFormat          *byte
	MessageExpiry          *uint32
	WillDelayInterval      *uint32
	ContentType            string
	ResponseTopic          string
	CorrelationData        []byte
	TopicAlias             *uint16
	SubscriptionIdentifier []int
	User                   []User
}

// ConnectProperties 保存 CONNECT 报文可变报头中的 MQTT 5 属性。
type ConnectProperties struct {
	SessionExpiryInterval *uint32
	AuthMethod            string
	AuthData              []byte
	RequestProblemInfo    *byte
	RequestResponseInfo   *byte
	ReceiveMaximum        *uint16
	TopicAliasMaximum     *uint16
	MaximumPacketSize     *uint32
	User                  []User
}

// ConnAckProperties 保存 CONNACK 报文可变报头中的 MQTT 5 属性。
type ConnAckProperties struct {
	SessionExpiryInterval *uint32
	AssignedClientID      string
	ServerKeepAlive       *uint16
	AuthMethod            string
	AuthData              []byte
	ResponseInfo          string
	ServerReference       string
	ReasonString          string
	ReceiveMaximum        *uint16
	TopicAliasMaximum     *uint16
	MaximumQOS            *byte
	RetainAvailable       *byte
	WildcardSubAvailable  *byte
	SubIDAvailable        *byte
	SharedSubAvailable    *byte
	MaximumPacketSize     *uint32
	User                  []User
}

// SubscribeProperties 保存 SUBSCRIBE 报文可变报头中的 MQTT 5 属性。
type SubscribeProperties struct {
	SubscriptionIdentifier *int
	User                   []User
}

// SubackProperties 保存 SUBACK 报文中的原因字符串和用户属性。
type SubackProperties struct {
	ReasonString string
	User         []User
}

// UnsubscribeProperties 保存 UNSUBSCRIBE 报文中的用户属性。
type UnsubscribeProperties struct {
	User []User
}

// UnsubackProperties 保存 UNSUBACK 报文中的原因字符串和用户属性。
type UnsubackProperties struct {
	ReasonString string
	User         []User
}

// PubackProperties 保存 PUBACK 报文中的原因字符串和用户属性。
type PubackProperties struct {
	ReasonString string
	User         []User
}

// PubrecProperties 保存 PUBREC 报文中的原因字符串和用户属性。
type PubrecProperties struct {
	ReasonString string
	User         []User
}

// PubrelProperties 保存 PUBREL 报文中的原因字符串和用户属性。
type PubrelProperties struct {
	ReasonString string
	User         []User
}

// PubcompProperties 保存 PUBCOMP 报文中的原因字符串和用户属性。
type PubcompProperties struct {
	ReasonString string
	User         []User
}

// DisconnectProperties 保存 DISCONNECT 报文中的 MQTT 5 属性。
type DisconnectProperties struct {
	SessionExpiryInterval *uint32
	ServerReference       string
	ReasonString          string
	User                  []User
}

// AuthProperties 保存 AUTH 扩展认证报文中的 MQTT 5 属性。
type AuthProperties struct {
	AuthMethod   string
	AuthData     []byte
	ReasonString string
	User         []User
}

func (p *PublishProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *ConnectProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *ConnAckProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *SubscribeProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *SubackProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *UnsubscribeProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *UnsubackProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *PubackProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *PubrecProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *PubrelProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *PubcompProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *DisconnectProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *AuthProperties) GetUserProperties() []User {
	if p == nil {
		return nil
	}
	return p.User
}
func (p *PublishProperties) SetUserProperties(users []User) { p.User = users }
func (p *ConnectProperties) SetUserProperties(users []User) { p.User = users }
func (p *ConnAckProperties) SetUserProperties(users []User) { p.User = users }
func (p *SubscribeProperties) SetUserProperties(users []User) {
	p.User = users
}
func (p *SubackProperties) SetUserProperties(users []User)      { p.User = users }
func (p *UnsubscribeProperties) SetUserProperties(users []User) { p.User = users }
func (p *UnsubackProperties) SetUserProperties(users []User)    { p.User = users }
func (p *PubackProperties) SetUserProperties(users []User)      { p.User = users }
func (p *PubrecProperties) SetUserProperties(users []User)      { p.User = users }
func (p *PubrelProperties) SetUserProperties(users []User)      { p.User = users }
func (p *PubcompProperties) SetUserProperties(users []User)     { p.User = users }
func (p *DisconnectProperties) SetUserProperties(users []User)  { p.User = users }
func (p *AuthProperties) SetUserProperties(users []User)        { p.User = users }

// String 只输出 User Property，避免调试日志里塞入过长的二进制属性。
func (p *PublishProperties) String() string     { return propertyString(p.User) }
func (p *ConnectProperties) String() string     { return propertyString(p.User) }
func (p *ConnAckProperties) String() string     { return propertyString(p.User) }
func (p *SubscribeProperties) String() string   { return propertyString(p.User) }
func (p *SubackProperties) String() string      { return propertyString(p.User) }
func (p *UnsubscribeProperties) String() string { return propertyString(p.User) }
func (p *UnsubackProperties) String() string    { return propertyString(p.User) }
func (p *PubackProperties) String() string      { return propertyString(p.User) }
func (p *PubrecProperties) String() string      { return propertyString(p.User) }
func (p *PubrelProperties) String() string      { return propertyString(p.User) }
func (p *PubcompProperties) String() string     { return propertyString(p.User) }
func (p *DisconnectProperties) String() string  { return propertyString(p.User) }
func (p *AuthProperties) String() string        { return propertyString(p.User) }

// propertyString 将 User Property 渲染成多行调试文本。
func propertyString(users []User) string {
	if len(users) == 0 {
		return ""
	}
	var b strings.Builder
	for _, user := range users {
		fmt.Fprintf(&b, "\t%s:%s\n", user.Key, user.Value)
	}
	return b.String()
}
