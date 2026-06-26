package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ClientOnline = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "skytree_mqtt_clients_online",
		Help: "Current number of online MQTT clients.",
	})

	ClientConnectEvent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_client_connect_total",
		Help: "Total number of MQTT client connect attempts.",
	})

	ClientConnectSuccessEvent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_client_connect_success_total",
		Help: "Total number of successful MQTT client connects.",
	})

	ClientConnectFailedEvent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_client_connect_failures_total",
		Help: "Total number of failed MQTT client connects.",
	})

	ClientDisconnectRequestEvent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_client_disconnect_total",
		Help: "Total number of MQTT client disconnects.",
	})
)
