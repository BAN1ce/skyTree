package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SubCenterRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_subscription_center_requests_total",
		Help: "Total number of subscription center requests.",
	}, []string{"operation", "result"})

	SubCenterRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_mqtt_subscription_center_request_duration_seconds",
		Help:    "Duration of subscription center requests.",
		Buckets: defaultDurationBuckets,
	}, []string{"operation", "result"})

	SubTreeReadNoSubscriber = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_subscription_center_no_subscriber_total",
		Help: "Total subscription center reads that found no subscribers.",
	})
)

func RecordSubCenterRequest(operation string, err error, duration time.Duration) {
	operation = normalizeSubCenterOperation(operation)
	result := resultFromError(err)
	SubCenterRequestsTotal.WithLabelValues(operation, result).Inc()
	SubCenterRequestDurationSeconds.WithLabelValues(operation, result).Observe(durationSeconds(duration))
}

func normalizeSubCenterOperation(operation string) string {
	switch operation {
	case "read", "write", "delete":
		return operation
	default:
		return unknownLabel
	}
}
