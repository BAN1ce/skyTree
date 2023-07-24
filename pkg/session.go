package pkg

import (
	"time"
)

type SessionKey string

type Session interface {
	SessionTopic
	Release() error
}

type SessionTopic interface {
	ReadSubTopics() map[string]int32
	CreateSubTopic(topic string, qos int32)
	DeleteSubTopic(topic string)
	SessionTopicMessage
}

type SessionTopicMessage interface {
	/*
		about un ack message id
	*/
	ReadTopicUnAckMessageID(topic string) []string
	CreateTopicUnAckMessageID(topic string, messageID []string)
	DeleteTopicUnAckMessageID(topic string, messageID string)
	/*
		about un rec packet id
	*/
	ReadTopicUnRecPacketID(topic string) []string
	CreateTopicUnRecPacketID(topic string, packetID []string)
	DeleteTopicUnRecPacketID(topic string, packetID string)

	/*
		about un comp packet id
	*/
	ReadTopicUnCompPacketID(topic string) []string
	CreateTopicUnCompPacketID(topic string, packetID []string)
	DeleteTopicUnCompPacketID(topic string, packetID string)

	/*
		about last acked message id
	*/

	ReadTopicLastAckedMessageID(topic string) (string, bool)
	SetTopicLastAckedMessageID(topic string, messageID string)
	DeleteTopicLastAckedMessageID(topic string, messageID string)
}

type SessionMeta interface {
	SetLastAliveTime(time time.Time)
}

type SessionExpiry interface {
	GetSessionExpiryInterval() uint32
	SetSessionExpiryInterval(uint32)
}

// func SessionSetWillFlag(client.proto Session, willFlag string) {
// 	client.proto.Set(WillFlag, willFlag)
// }
//
// func SetWillPropertyToSession(client.proto Session, properties *packets.Properties) {
// 	client.proto.Set(WillPropertyMessageExpiryInterval, cast.ToString(properties.MessageExpiry))
// 	client.proto.Set(WillPropertyWillDelayInterval, cast.ToString(properties.WillDelayInterval))
// 	client.proto.Set(WillPropertyPayloadFormatIndicator, cast.ToString(properties.PayloadFormat))
// 	client.proto.Set(WillPropertyContentType, cast.ToString(properties.ContentType))
// 	client.proto.Set(WillPropertyResponseTopic, cast.ToString(properties.ResponseTopic))
// 	client.proto.Set(WillPropertyCorrelationData, cast.ToString(properties.CorrelationData))
// 	client.proto.Set(WillPropertySubscriptionIdentifier, cast.ToString(properties.SubscriptionIdentifier))
// 	for _, v := range properties.User {
// 		client.proto.Set(SessionKey(fmt.Sprintf("%s,%s", WillPropertyUserProperty, v.Key)), cast.ToString(v.Value))
// 	}
// }
