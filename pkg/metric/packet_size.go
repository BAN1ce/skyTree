package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DiscardOversizedOutboundPublish = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_discard_oversized_outbound_publish_total",
		Help: "Number of outbound PUBLISH packets discarded because they exceed the client's Maximum Packet Size",
	}, []string{"qos"})
)
