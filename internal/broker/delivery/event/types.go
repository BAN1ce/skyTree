package delivery_event

import brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"

// Kind describes how the client delivery runner should react to a notification.
type Kind int32

const (
	KindUnspecified Kind = 0
	KindWake        Kind = 1
	KindQoS0Direct  Kind = 2
	KindSharedWake  Kind = 3
)

// Notify is the payload emitted on the local EventCenter for a specific clientID.
//   - KindWake: Message is nil, client runner should re-check store (with backoff).
//   - KindQoS0Direct: Message is a decoded broker publish message for direct QoS0 delivery (no persistence).
//     NoLocal and RAP are MQTT5 subscription options applied during delivery.
type Notify struct {
	Kind         Kind
	PublishTopic string
	Message      *brokerpublish.Message
	// NoLocal: if true, do not deliver to the same client that published the message (MQTT5).
	// Only used when Kind == KindQoS0Direct.
	NoLocal bool
	// RAP (Retain As Published): if false, force retain=0 for this delivery (MQTT5).
	// Only used when Kind == KindQoS0Direct.
	RAP bool
	// SubscriptionIDsJSON: JSON array of subscription identifiers for matched subscriptions (MQTT5).
	// Only used when Kind == KindQoS0Direct.
	SubscriptionIDsJSON string
}
