package event

import "github.com/BAN1ce/skyTree/inner/broker/event"

var GlobalEvent = &Event{}

type Event struct {
}

func (e *Event) CreatePublishEvent(topic string, handler func(...interface{})) {
	event.Event.AddListener(event.ReceivedTopicPublishEventName(topic), handler)

}

func (e *Event) DeletePublishEvent(topic string, handler func(i ...interface{})) {
	event.Event.RemoveListener(event.ReceivedTopicPublishEventName(topic), handler)
}

func (e *Event) EmitStoreMessage(topic, messageID string) {
	event.Event.Emit(event.TopicMessageStoredEventName(topic), topic, messageID)
}
func (e *Event) CreateListenMessageStoreEvent(topic string, handler func(...interface{})) {
	event.Event.AddListener(event.TopicMessageStoredEventName(topic), handler)
}

func (e *Event) DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{})) {
	event.Event.RemoveListener(event.TopicMessageStoredEventName(topic), handler)
}
