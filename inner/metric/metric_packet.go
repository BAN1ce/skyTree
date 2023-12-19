package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	Received = promauto.NewCounter(prometheus.CounterOpts{
		Name: "received",
		Help: "",
	})

	Send = promauto.NewCounter(prometheus.CounterOpts{
		Name: "send",
		Help: "",
	})

	ReceivedBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "received_bytes",
	})

	SendBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "send_bytes",
	})

	ReceivedError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "received_error",
	})

	SendError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "send_error",
	})

	ConnectReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "connect_received",
	})

	ConnectAckSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "connect_ack_send",
	})

	PublishReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publish_received",
	})

	PublishSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publish_send",
	})

	PublishAckSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publish_ack_send",
	})

	PublishAckReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publish_ack_received",
	})

	PubRecReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubrec_received",
	})

	PubRecSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubrec_send",
	})

	PubRelReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubrel_received",
	})

	PubRelSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubrel_send",
	})

	PubCompReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubcomp_received",
	})

	PubCompSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pubcomp_send",
	})

	SubscribeReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "subscribe_received",
	})

	UnsubscribeReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "unsubscribe_received",
	})

	SubscribeAckSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "subscribe_ack_send",
	})

	UnsubscribeAckSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "unsubscribe_ack_send",
	})

	PingReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ping_received",
	})

	PongSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pong_send",
	})

	DisconnectReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "disconnect_received",
	})

	DisconnectSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "disconnect_send",
	})

	AuthReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_received",
	})

	AuthSend = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_send",
	})
)
