package broker

import "github.com/eclipse/paho.golang/packets"

type SessionKey string

type Session interface {
	SessionTopic
	Release()
	SessionWillMessage
	SessionCreateConnectProperties
}

type SessionTopic interface {
	ReadSubTopics() (topics map[string]int32)
	CreateSubTopic(topic string, qos int32)
	DeleteSubTopic(topic string)
	SessionTopicMessage
}

type SessionTopicMessage interface {
	SessionTopicUnFinishedMessage
	SessionTopicLatestPushedMessage
}

type UnFinishedMessage struct {
	MessageID   string
	PacketID    string
	PubReceived bool
}

// SessionTopicUnFinishedMessage save the unfinished message for topic
type SessionTopicUnFinishedMessage interface {
	CreateTopicUnFinishedMessage(topic string, message []UnFinishedMessage)
	ReadTopicUnFinishedMessage(topic string) (message []UnFinishedMessage)
	DeleteTopicUnFinishedMessage(topic string, messageID string)
}

// SessionTopicLatestPushedMessage save the latest pushed messageID for topic
type SessionTopicLatestPushedMessage interface {
	ReadTopicLatestPushedMessageID(topic string) (messageID string, ok bool)
	SetTopicLatestPushedMessageID(topic string, messageID string)
	DeleteTopicLatestPushedMessageID(topic string, messageID string)
}

type PropertyKey = string

type SessionConnectProperties struct {
	ExpiryInterval    int64  `json:"expiry_interval"`
	ReceiveMaximum    uint16 `json:"receive_maximum"`
	MaximumPacketSize uint32 `json:"maximum_packet_size"`
	TopicAliasMaximum uint16 `json:"topic_alias_maximum"`
	RequestResponse   bool   `json:"request_response"`
	RequestProblem    bool   `json:"request_problem"`
}

type SessionCreateConnectProperties interface {
	GetConnectProperties() (*SessionConnectProperties, error)
	SetConnectProperties(properties *SessionConnectProperties) error
}

type UserProperties = packets.User

type WillProperties struct {
	WillDelayInterval int64            `json:"will_delay_interval"`
	PayloadFormat     int64            `json:"payload_format"`
	ExpiryInterval    int64            `json:"expiry_interval"`
	ContentType       string           `json:"content_type"`
	ResponseTopic     string           `json:"response_topic"`
	CorrelationData   []byte           `json:"correlation_data"`
	UserProperties    []UserProperties `json:"user_properties"`
}
type WillMessage struct {
	Topic    string         `json:"topic"`
	Payload  []byte         `json:"payload"`
	QoS      uint8          `json:"qos"`
	Retain   bool           `json:"retain"`
	Property WillProperties `json:"property"`
}
type SessionWillMessage interface {
	GetWillMessage() (*WillMessage, error)
	SetWillMessage(message *WillMessage) error
}

func ConnectPacketToWillMessage(connect *packets.Connect) *WillMessage {
	return &WillMessage{
		Topic:   connect.WillTopic,
		Payload: connect.WillMessage,
		QoS:     connect.WillQOS,
		Retain:  connect.WillRetain,
		Property: WillProperties{
			WillDelayInterval: int64(*connect.WillProperties.WillDelayInterval),
			PayloadFormat:     int64(*connect.WillProperties.PayloadFormat),
			ExpiryInterval:    int64(*connect.WillProperties.MessageExpiry),
			ContentType:       connect.WillProperties.ContentType,
			ResponseTopic:     connect.WillProperties.ResponseTopic,
			CorrelationData:   connect.WillProperties.CorrelationData,
			UserProperties:    connect.WillProperties.User,
		},
	}

}
