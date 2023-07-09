package pkg

import (
	"time"
)

type SessionKey string

type Session interface {
	Destroy()
	SessionTopic
}

type SessionClient interface {
	GetSubTopics() map[string]int32
	CreateSubTopic(topic string, qos int32)
	DeleteSubTopic(topic string)
	SessionTopicMessage
}

type SessionTopicMessage interface {
	/*
		about un ack message id
	*/
	ReadTopicUnAckMessageID(topic string) []string
	SetTopicUnAckMessageID(topic string, messageID []string)
	DeleteTopicUnAckMessageID(topic string)
	/*
		about un rec packet id
	*/
	ReadTopicUnRecPacketID(topic string) []string
	SetTopicUnRecPacketID(topic string, packetID []string)
	DeleteTopicUnRecPacketID(topic string)

	/*
		about un comp packet id
	*/
	ReadTopicUnCompPacketID(topic string) []string
	SetTopicUnCompPacketID(topic string, packetID []string)
	DeleteTopicUnCompPacketID(topic string)

	/*
		about last acked message id
	*/

	ReadTopicLastAckedMessageID(topic string) (string, bool)
	SetTopicLastAckedMessageID(topic string, messageID string)
	DeleteTopicLastAckedMessageID(topic string)
}

type SessionTopic interface {
	ReleaseTopicSession(topic string)

	CreateWill(topic string, qos int32, retain bool, payload []byte, properties map[string]string)
	// CreateSubTopics create sub topic with QoS
	CreateSubTopics(topic string, qos int32)

	// SaveTopicUnAckMessageID ReadTopicUnAckMessageID read topic un ack message id when client connect again
	SaveTopicUnAckMessageID(topic string, messageID []string)

	SaveTopicUnRecPacketID(topic string, packetID []string)

	// UpdateTopicLastAckedMessageID update topic last acked message id
	UpdateTopicLastAckedMessageID(topic string, messageID string)

	// ReadSubTopics read all sub topics
	ReadSubTopics() map[string]int32

	// ReadTopicLastAckedMessageID read topic last acked message id for client read next message id
	ReadTopicLastAckedMessageID(topic string) (string, bool)

	// ReadTopicUnAckMessageID  read topic un ack message id when client connect again
	ReadTopicUnAckMessageID(topic string) []string

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
