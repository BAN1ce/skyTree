package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WillMessageSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_will_send",
	})

	MessageQoS0Received = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos0_received",
	})

	MessageQoS0Send = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos0_send",
	})

	MessageQoS1Received = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos1_received",
	})

	MessageQoS1Send = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos1_send",
	})

	MessageQoS2Received = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos2_received",
	})

	MessageQoS2Send = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_qos2_send",
	})

	MessageDropWithNoSubscribers = promauto.NewCounter(prometheus.CounterOpts{
		Name: "message_drop_with_no_subscribers",
	})
)
