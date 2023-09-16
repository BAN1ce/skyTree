package event

import (
	"github.com/kataras/go-events"
)

var (
	Driver = events.New()
)

const (
	Connect               = "event.connect"
	Disconnect            = "event.disconnect"
	Ping                  = "event.ping"
	Pong                  = "event.pong"
	Subscribe             = "event.subscribe"
	Unsubscribe           = "event.unsubscribe"
	ClientPublish         = "event.client.publish"
	ClientPublishTopic    = "event.client.publish_topic"
	BrokerPublish         = "event.store.publish"
	BrokerPublishToClient = "event.store.publish_to_client"
	ClientPublishAck      = "event.client.publish_ack"
	BrokerPublishAck      = "event.store.publish_ack"
	ClientAuth            = "event.client.auth"
	BrokerAuth            = "event.store.auth"
)

func WithEventPrefix(name, s string) events.EventName {
	return events.EventName(name + "." + s)
}

func ReceivedTopicPublishEventName(topic string) events.EventName {
	return WithEventPrefix(ClientPublishTopic, topic)
}
