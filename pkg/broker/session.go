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
	SessionTopicUnFinishedMessage
	SessionTopicLatestPushedMessage
}

type UnFinishedMessage struct {
	MessageID   string
	PacketID    string
	PubReceived bool
}

type SessionTopicUnFinishedMessage interface {
	CreateTopicUnFinishedMessage(topic string, message []UnFinishedMessage)
	ReadTopicUnFinishedMessage(topic string) (message []UnFinishedMessage)
	DeleteTopicUnFinishedMessage(topic string, messageID string)
}

type SessionTopicLatestPushedMessage interface {
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
