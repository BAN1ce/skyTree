package event

import (
	"fmt"
	"github.com/kataras/go-events"
)

var (
	event = events.New()
)

const (
	EventPublish         = "event_publish"
	eventPublishToClient = "event_publish_to_client"
)

func clientPublishToClientEventName(clientID string) events.EventName {
	return events.EventName(fmt.Sprintf("%s_%s", eventPublishToClient, clientID))
}
