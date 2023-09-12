package qos1

import "github.com/BAN1ce/skyTree/pkg/broker"

type meta struct {
	topic           string
	qos             byte
	windowSize      int
	writer          broker.PublishWriter
	latestMessageID string
}

type Session interface {
	broker.SessionTopicLatestPushedMessage
	broker.SessionTopicUnFinishedMessage
}
