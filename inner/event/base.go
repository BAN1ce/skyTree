package event

import "github.com/BAN1ce/skyTree/inner/broker/event"

var GloablEvent = &Event{}

type Event struct {
}

func (e *Event) CreatePublishEvent(topic string, handler func(...interface{})) {
	event.Event.AddListener(event.ReceivedTopicPublishEventName(topic), handler)

}

func (e *Event) DeletePublishEvent(topic string, handler func(i ...interface{})) {
	event.Event.RemoveListener(event.ReceivedTopicPublishEventName(topic), handler)
}
