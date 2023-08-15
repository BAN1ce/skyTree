package broker

import (
	"time"
)

type SessionKey string

type Session interface {
	SessionTopic
	Release()
}

type SessionTopic interface {
	ReadSubTopics() (topics map[string]int32)
	CreateSubTopic(topic string, qos int32)
	DeleteSubTopic(topic string)
	SessionTopicMessage
}

type SessionTopicMessage interface {
	/*
		about un ack message id
	*/
	ReadTopicUnAckMessageID(topic string) (id []string)
	CreateTopicUnAckMessageID(topic string, messageID []string)
	DeleteTopicUnAckMessageID(topic string, messageID string)
	/*
		about un rec packet id
	*/
	ReadTopicUnRecPacketID(topic string) (packetID []string)
	CreateTopicUnRecPacketID(topic string, packetID []string)
	DeleteTopicUnRecPacketID(topic string, packetID string)

	/*
		about un comp packet id
	*/
	ReadTopicUnCompPacketID(topic string) (packetID []string)
	CreateTopicUnCompPacketID(topic string, packetID []string)
	DeleteTopicUnCompPacketID(topic string, packetID string)

	/*
		about last acked message id
	*/

	ReadTopicLatestPushedMessageID(topic string) (messageID string, ok bool)
	SetTopicLatestPushedMessageID(topic string, messageID string)
	DeleteTopicLatestPushedMessageID(topic string, messageID string)
}

type SessionMeta interface {
	SetLastAliveTime(time time.Time)
}

type SessionExpiry interface {
	GetSessionExpiryInterval() uint32
	SetSessionExpiryInterval(uint32)
}
