package event

import (
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"github.com/kataras/go-events"
)

var (
	Driver = events.New()
)

var GlobalEvent *Event

func Boot() {
	GlobalEvent = &Event{}
	createClientMetricListen()
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

func (e *Event) EmitClientPublish(topic string, publish *packet.Message) {
	Driver.Emit(ReceivedTopicPublishEventName(topic), topic, publish)
}

func (e *Event) EmitMQTTPacket(eventName MQTTEventName, packet packets.Packet) {
	Driver.Emit(events.EventName(eventName), packet)
}
