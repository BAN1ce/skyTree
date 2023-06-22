package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
//	SentPublishCount = promauto.NewCounter(prometheus.CounterOpts{
//		Name: "skytree.mqtt.publish.count.sent",
//		Help: "The total number of published messages to client",
//	})
//
//	SentTopicPublishCount = promauto.NewCounterVec(prometheus.CounterOpts{
//		Name: "skytree.mqtt.topic.publish.count.sent",
//		Help: "The total number of published messages belong a topic to client",
//	}, []string{"topic"})
)
var (
	ReceivedPublishCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree.mqtt.publish.count.received",
		Help: "The total number of received publish messages from client",
	})

	ReceiveTopicPublishCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree.mqtt.topic.publish.count.received",
		Help: "The total number of received publish messages belong a topic from client",
	}, []string{"topic"})
)

// TODO: add more metrics

func MountMetric() {

}
