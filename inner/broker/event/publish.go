package event

import "github.com/BAN1ce/skyTree/logger"

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
