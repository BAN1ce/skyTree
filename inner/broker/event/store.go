package event

import "github.com/kataras/go-events"

const (
	MessageStored = "event.topic.message_stored"
)

func TopicMessageStoredEventName(topic string) events.EventName {
	return WithEventPrefix(MessageStored, topic)
}
