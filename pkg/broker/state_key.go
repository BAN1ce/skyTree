package broker

import (
	"github.com/BAN1ce/skyTree/logger"
	"go.uber.org/zap"
	"strings"
)

const (
	KeyTopicPrefix        = "topic/"
	KeyTopicWillMessage   = `/will_message`
	KeyTopicRetainMessage = `/retain_message`
)

func TopicKey(topic string) *strings.Builder {
	var build = strings.Builder{}
	build.WriteString(KeyTopicPrefix)
	build.WriteString(topic)
	return &build
}

func TopicWillMessage(topic string) *strings.Builder {
	var build = TopicKey(topic)
	build.WriteString(KeyTopicWillMessage)
	return build
}

func TopicWillMessageMessageIDKey(topic, messageID string) *strings.Builder {
	var build = TopicWillMessage(topic)
	build.WriteString("/")
	build.WriteString(messageID)
	logger.Logger.Debug("TopicWillMessageMessageIDKey", zap.String("build", build.String()))
	return build
}

func TrimTopicWillMessageIDKey(topic, key string) string {
	var build = TopicWillMessage(topic)
	build.WriteString("/")
	logger.Logger.Debug("TrimTopicWillMessageIDKey", zap.String("key", key), zap.String("build", build.String()))
	return strings.TrimPrefix(key, build.String())
}

func TopicRetainMessage(topic string) *strings.Builder {
	var build = TopicKey(topic)
	build.WriteString(KeyTopicRetainMessage)
	return build
}

func TopicRetainMessageMessageIDKey(topic, messageID string) *strings.Builder {
	var build = TopicRetainMessage(topic)
	build.WriteString("/")
	build.WriteString(messageID)
	return build
}
