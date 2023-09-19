package broker

import "strings"

const (
	KeyTopicPrefix      = "topic/"
	KeyTopicWillMessage = `/will_message`
)

func TopicKey(topic string) *strings.Builder {
	var build = strings.Builder{}
	build.WriteString(KeyTopicPrefix)
	build.WriteString(topic)
	return &build
}
func TopicWillMessage(topic string) string {
	var build = TopicKey(topic)
	build.WriteString(KeyTopicWillMessage)
	return build.String()
}

func TopicWillMessageMessageIDKey(topic, messageID string) string {
	var build = TopicKey(topic)
	build.WriteString(KeyTopicWillMessage)
	build.WriteString("/")
	build.WriteString(messageID)
	return build.String()
}
