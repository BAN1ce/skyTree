package event

import "github.com/kataras/go-events"

type Name string

const (
	PublishQoS0Prefix = Name("event_publish_qos0")
)

func WithEventPrefix(name Name, s string) events.EventName {
	return events.EventName(string(name) + "_" + s)
}
