package metric

import (
	"github.com/BAN1ce/skyTree/inner/broker/event"
	"github.com/BAN1ce/skyTree/logger"
)

func Init() {
	event.Event.AddListener(event.ClientPublish, func(i ...interface{}) {
		if len(i) > 0 {
			topic, ok := i[0].(string)
			if !ok {
				logger.Logger.Error("ListenTopicPublishEvent: type error")
				return
			}
			ReceiveTopicPublishCount.With(map[string]string{"topic": topic}).Inc()
			ReceivedPublishCount.Inc()
		}
	})
}
