package pkg

import (
	"fmt"
	"github.com/eclipse/paho.golang/packets"
	"github.com/spf13/cast"
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
	WillFlagTrue    = "true"
	WillFlagFalse   = "false"
	WillQos0        = "0"
	WillQos1        = "1"
	WillQos2        = "2"
	WillRetainTrue  = "true"
	WillRetainFalse = "false"
)

type Session interface {
	Get(key SessionKey) string
	Set(key SessionKey, value string)
	GetWithPrefix(prefix string, keyWithPrefix bool) map[string]string
	Destroy()
}

func SessionSetWillFlag(session Session, willFlag string) {
	session.Set(WillFlag, willFlag)
}

func SetPropertyToSession(session Session, properties *packets.Properties) {
	session.Set(PropertySessionExpiryInterval, cast.ToString(properties.SessionExpiryInterval))
	session.Set(PropertyReceiveMaximum, cast.ToString(properties.ReceiveMaximum))
	session.Set(PropertyMaximumPacketSize, cast.ToString(properties.MaximumPacketSize))
	session.Set(PropertyMaximumQoS, cast.ToString(properties.MaximumQOS))
	session.Set(PropertyTopicAliasMaximum, cast.ToString(properties.TopicAliasMaximum))
	session.Set(PropertyRequestResponseInfo, cast.ToString(properties.RequestResponseInfo))
	session.Set(PropertyRequestProblemInfo, cast.ToString(properties.RequestProblemInfo))
	for _, v := range properties.User {
		session.Set(SessionKey(fmt.Sprintf("%s.%s", PropertyUserProperty, v.Key)), cast.ToString(v.Value))
	}
}

func SetWillPropertyToSession(session Session, properties *packets.Properties) {
	session.Set(WillPropertyMessageExpiryInterval, cast.ToString(properties.MessageExpiry))
	session.Set(WillPropertyWillDelayInterval, cast.ToString(properties.WillDelayInterval))
	session.Set(WillPropertyPayloadFormatIndicator, cast.ToString(properties.PayloadFormat))
	session.Set(WillPropertyContentType, cast.ToString(properties.ContentType))
	session.Set(WillPropertyResponseTopic, cast.ToString(properties.ResponseTopic))
	session.Set(WillPropertyCorrelationData, cast.ToString(properties.CorrelationData))
	session.Set(WillPropertySubscriptionIdentifier, cast.ToString(properties.SubscriptionIdentifier))
	for _, v := range properties.User {
		session.Set(SessionKey(fmt.Sprintf("%s.%s", WillPropertyUserProperty, v.Key)), cast.ToString(v.Value))
	}
}
