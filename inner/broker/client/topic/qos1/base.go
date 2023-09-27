package qos1

import (
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/skyTree/pkg/broker"
)

type meta struct {
	topic           string
	subOption       *proto.SubOption
	windowSize      int
	writer          broker.PublishWriter
	latestMessageID string
}

type Session interface {
	broker.SessionTopicLatestPushedMessage
	broker.SessionTopicUnFinishedMessage
}
