package eventbus

// Centralized event name definitions and helpers.
// Keep all event name strings in one place to avoid inconsistencies across packages.

const (
	// ReceivedClientDeliveryEventNamePrefix is the local event name prefix for client delivery wake/direct events.
	// Event name format: "event.delivery.task.client." + <clientID>
	ReceivedClientDeliveryEventNamePrefix = "event.delivery.task.client."
)

// ReceiveClientDeliveryEventName returns the local event name for client delivery events.
func ReceiveClientDeliveryEventName(clientID string) string {
	return ReceivedClientDeliveryEventNamePrefix + clientID
}
