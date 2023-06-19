package event

import (
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

func EmitClientPublishTopicEvent(topic string, publish *packets.Publish) {
	Event.Emit(WithEventPrefix(ClientPublishTopic, topic), topic, publish)
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
	Event.AddListener(WithEventPrefix(ClientPublishTopic, topic), listener)
	return listener
}

func RemoveTopicPublishEvent(topic string, listener func(i ...interface{})) {
	Event.RemoveListener(WithEventPrefix(ClientPublishTopic, topic), listener)
}
