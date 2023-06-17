package event

import (
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/BAN1ce/skyTree/logger"
	event2 "github.com/BAN1ce/skyTree/pkg/event"
	"github.com/eclipse/paho.golang/packets"
)

func EmitPublishToClientEvent(clientID, topic, messageID string) {
	event.Emit(clientPublishToClientEventName(clientID), topic, messageID)
}

func ListenPublishToClientEvent(clientID string, f func(topic, id string)) {
	event.Once(clientPublishToClientEventName(clientID), func(i ...interface{}) {
		if len(i) > 0 {
			t, ok := i[0].(string)
			if !ok {
				logger.Logger.Error("ListenPublishToClientEvent: type error")
				return
			}
			id, ok := i[1].(string)
			if ok {
				f(t, id)
			} else {
				logger.Logger.Error("ListenPublishToClientEvent: type error")
			}
		}
	})
}

func EmitTopicPublishEvent(topic string, publish *packets.Publish) {
	event.Emit(event2.WithEventPrefix(event2.PublishQoS0Prefix, topic), topic, publish)
	metric.TopicReceivedPublishCount.With(map[string]string{"topic": topic}).Inc()
	metric.ReceivedPublishCount.Inc()
}

func ListenTopicPublishEvent(topic string, f func(topic string, publish *packets.Publish)) func(i ...interface{}) {
	listener := func(i ...interface{}) {
		if len(i) == 2 {
			t, ok := i[0].(string)
			if !ok {
				logger.Logger.Error("ListenTopicPublishEvent: type error")
				return
			}
			p, ok := i[1].(*packets.Publish)
			if ok {
				f(t, p)
			} else {
				logger.Logger.Error("ListenTopicPublishEvent: type error")
			}
		}
	}
	event.AddListener(event2.WithEventPrefix(event2.PublishQoS0Prefix, topic), listener)
	return listener
}

func RemoveTopicPublishEvent(topic string, listener func(i ...interface{})) {
	event.RemoveListener(event2.WithEventPrefix(event2.PublishQoS0Prefix, topic), listener)
}
