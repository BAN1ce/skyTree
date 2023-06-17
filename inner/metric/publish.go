package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PublishCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_count",
		Help: "The total number of published messages to client",
	})
	ReceivedPublishCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_received_publish_count",
		Help: "The total number of received publish messages from client",
	})
)
var (
	TopicPublishCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_topic_count",
		Help: "The total number of published messages belong a topic to client",
	}, []string{"topic"})
	TopicReceivedPublishCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_received_publish_topic_count",
		Help: "The total number of received publish messages belong a topic from client",
	}, []string{"topic"})
)
