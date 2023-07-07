package metric

import (
	"github.com/BAN1ce/skyTree/inner/broker/event"
)

func Init() {
	event.Event.AddListener(event.ClientPublish, func(i ...interface{}) {
		if len(i) > 0 {
		}
	})
}
