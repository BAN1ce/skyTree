package pkg

import (
	"context"
	"time"
)

type SessionKey string

const (
	WillFlag      = SessionKey("will_flag")
	WillQos       = SessionKey("will_qos")
	WillRetain    = SessionKey("will_retain")
	WillMessage   = SessionKey("will_message")
	Username      = SessionKey("username")
	KeepAlive     = SessionKey("keep_alive")
	LastAliveTime = SessionKey("last_alive_time")
)
const (
	PropertySessionExpiryInterval = SessionKey("session_expiry_interval")
	PropertyReceiveMaximum        = SessionKey("receive_maximum")
	PropertyMaximumPacketSize     = SessionKey("maximum_packet_size")
	PropertyMaximumQoS            = SessionKey("maximum_qos")
	PropertyTopicAliasMaximum     = SessionKey("topic_alias_maximum")
	PropertyRequestResponseInfo   = SessionKey("request_response_info")
	PropertyRequestProblemInfo    = SessionKey("request_problem_info")
	PropertyUserProperty          = SessionKey("user_property")
	PropertyAuthMethod            = SessionKey("auth_method")
	PropertyAuthData              = SessionKey("auth_data")
)

const (
	WillPropertyWillDelayInterval      = SessionKey("will_delay_interval")
	WillPropertyPayloadFormatIndicator = SessionKey("will_payload_format_indicator")
	WillPropertyMessageExpiryInterval  = SessionKey("will_message_expiry_interval")
	WillPropertyContentType            = SessionKey("will_content_type")
	WillPropertyResponseTopic          = SessionKey("will_response_topic")
	WillPropertyCorrelationData        = SessionKey("will_correlation_data")
	WillPropertySubscriptionIdentifier = SessionKey("will_subscription_identifier")
	WillPropertyUserProperty           = SessionKey("will_user_property")
	WillTopic                          = SessionKey("will_topic")
	WillPayload                        = SessionKey("will_payload")
)
const (
	SubTopic = SessionKey("sub_topic")
)

const (
	WillFlagTrue    = "true"
	WillFlagFalse   = "false"
	WillQos0        = "0"
	WillQos1        = "1"
	WillQos2        = "2"
	WillRetainTrue  = "true"
	WillRetainFalse = "false"
)

type Session interface {
	Destroy()
	SessionTopic
}

type SessionTopic interface {
	ReleaseTopicSession(topic string)

	OnceListenTopicStoreEvent(ctx context.Context, topic string, f func(topic, id string))

	CreateWill(topic string, qos int32, retain bool, payload []byte, properties map[string]string)
	// CreateSubTopics create sub topic with QoS
	CreateSubTopics(topic string, qos int32)

	// SaveTopicUnAckMessageID ReadTopicUnAckMessageID read topic un ack message id when client connect again
	SaveTopicUnAckMessageID(topic string, messageID []string)

	// UpdateTopicLastAckedMessageID update topic last acked message id
	UpdateTopicLastAckedMessageID(topic string, messageID string)

	// ReadSubTopics read all sub topics
	ReadSubTopics() map[string]int32

	// ReadTopicLastAckedMessageID read topic last acked message id for client read next message id
	ReadTopicLastAckedMessageID(topic string) (string, bool)

	// ReadTopicUnAckMessageID  read topic un ack message id when client connect again
	ReadTopicUnAckMessageID(topic string) []string

	ReadSubTopicsLastAckedMessageID() map[string]string

	// DeleteSubTopics delete sub topic
	DeleteSubTopics(topic string)

	// DeleteTopicUnAckMessageID delete topic un ack message id when client ack message
	DeleteTopicUnAckMessageID(topic string, messageID string)
}

type SessionMeta interface {
	SetLastAliveTime(time time.Time)
}
type SessionExpiry interface {
	GetSessionExpiryInterval() uint32
	SetSessionExpiryInterval(uint32)
}

// func SessionSetWillFlag(session Session, willFlag string) {
// 	session.Set(WillFlag, willFlag)
// }
//
// func SetWillPropertyToSession(session Session, properties *packets.Properties) {
// 	session.Set(WillPropertyMessageExpiryInterval, cast.ToString(properties.MessageExpiry))
// 	session.Set(WillPropertyWillDelayInterval, cast.ToString(properties.WillDelayInterval))
// 	session.Set(WillPropertyPayloadFormatIndicator, cast.ToString(properties.PayloadFormat))
// 	session.Set(WillPropertyContentType, cast.ToString(properties.ContentType))
// 	session.Set(WillPropertyResponseTopic, cast.ToString(properties.ResponseTopic))
// 	session.Set(WillPropertyCorrelationData, cast.ToString(properties.CorrelationData))
// 	session.Set(WillPropertySubscriptionIdentifier, cast.ToString(properties.SubscriptionIdentifier))
// 	for _, v := range properties.User {
// 		session.Set(SessionKey(fmt.Sprintf("%s,%s", WillPropertyUserProperty, v.Key)), cast.ToString(v.Value))
// 	}
// }
