package event

var GlobalEvent *Event

func Boot() {
	GlobalEvent = &Event{}
}

type Event struct {
}

func (e *Event) CreatePublishEvent(topic string, handler func(...interface{})) {
	Driver.AddListener(ReceivedTopicPublishEventName(topic), handler)
}

func (e *Event) DeletePublishEvent(topic string, handler func(i ...interface{})) {
	Driver.RemoveListener(ReceivedTopicPublishEventName(topic), handler)
}

func (e *Event) EmitStoreMessage(topic, messageID string) {
	Driver.Emit(TopicMessageStoredEventName(topic), topic, messageID)
}
func (e *Event) CreateListenMessageStoreEvent(topic string, handler func(...interface{})) {
	Driver.AddListener(TopicMessageStoredEventName(topic), handler)
}

func (e *Event) DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{})) {
	Driver.RemoveListener(TopicMessageStoredEventName(topic), handler)
}
