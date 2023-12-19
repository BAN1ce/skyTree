package session

import (
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"time"
)

// Session is the client session interface
type Session interface {
	TopicManager

	Release()

	GetWillMessage() (*WillMessage, bool, error)
	SetWillMessage(message *WillMessage) error
	DeleteWillMessage() error

	// Properties something

	GetConnectProperties() (*ConnectProperties, error)
	SetConnectProperties(properties *ConnectProperties) error

	SetExpiryInterval(int64)
	GetExpiryInterval() int64
}

// TopicManager save the session topic information
type TopicManager interface {
	TopicUnFinishedMessage

	TopicLatestPushedMessage

	ReadSubTopics() (topics map[string]*proto.SubOption)
	CreateSubTopic(topic string, option *proto.SubOption)
	DeleteSubTopic(topic string)
}

// TopicUnFinishedMessage save the unfinished message for topic
type TopicUnFinishedMessage interface {
	CreateTopicUnFinishedMessage(topic string, message []*packet.Message)

	// ReadTopicUnFinishedMessage read the unfinished message for topic, *packet.Message doesn't have the PublishPacket,PubRelPacket field.
	ReadTopicUnFinishedMessage(topic string) (message []*packet.Message)

	DeleteTopicUnFinishedMessage(topic string, messageID string)
}

// TopicLatestPushedMessage save the latest pushed messageID for topic
type TopicLatestPushedMessage interface {
	ReadTopicLatestPushedMessageID(topic string) (messageID string, ok bool)
	SetTopicLatestPushedMessageID(topic string, messageID string)
	DeleteTopicLatestPushedMessageID(topic string, messageID string)
}

type PropertyKey = string

type ConnectProperties struct {
	*packets.Properties
	CreatedTime int64 `json:"created_time"`
}

func (c *ConnectProperties) IsExpired() bool {
	if c.SessionExpiryInterval == nil {
		return false

	}
	if *c.MessageExpiry == 0xffffffff {
		return false
	}
	if int64(int64(*c.SessionExpiryInterval)+c.CreatedTime) > time.Now().Unix() {
		return false
	}
	return true
}

type UserProperties = packets.User

func NewConnectProperties(properties *packets.Properties) *ConnectProperties {
	return &ConnectProperties{
		Properties:  properties,
		CreatedTime: time.Now().Unix(),
	}
}
