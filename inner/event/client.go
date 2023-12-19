package event

import (
	"github.com/BAN1ce/skyTree/inner/metric"
)

const (
	ClientOnline  = "event.client.online"
	ClientOffline = "event.client.offline"
)

func (e *Event) EmitClientOnline(clientID string) {
	Driver.Emit(ClientOnline, clientID)
}

func (e *Event) CreateListenClientOnline(handler func(i ...interface{})) {
	Driver.AddListener(ClientOnline, handler)
}
func (e *Event) EmitClientOffline(clientID string) {
	Driver.Emit(ClientOffline, clientID)
}

func (e *Event) CreateListenClientOffline(handler func(i ...interface{})) {
	Driver.AddListener(ClientOffline, handler)
}

func createClientMetricListen() {
	GlobalEvent.CreateListenClientOnline(func(i ...interface{}) {
		if len(i) < 1 {
			return
		}
		if _, ok := i[0].(string); ok {
			metric.ClientOnline.Inc()
		}
	})

	GlobalEvent.CreateListenClientOffline(func(i ...interface{}) {
		if len(i) < 1 {
			return
		}
		if _, ok := i[0].(string); ok {
			metric.ClientOnline.Inc()
		}
	})

}
