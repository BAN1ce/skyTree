package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	StoreRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_store_requests_total",
		Help: "Total number of storage requests.",
	}, []string{"operation", "result"})

	StoreRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_store_request_duration_seconds",
		Help:    "Duration of storage requests.",
		Buckets: defaultDurationBuckets,
	}, []string{"operation", "result"})
)

func RecordStoreRequest(operation string, err error, duration time.Duration) {
	operation = normalizeStoreOperation(operation)
	result := resultFromError(err)
	StoreRequestsTotal.WithLabelValues(operation, result).Inc()
	StoreRequestDurationSeconds.WithLabelValues(operation, result).Observe(durationSeconds(duration))
}

func normalizeStoreOperation(operation string) string {
	switch operation {
	case "read", "write", "delete":
		return operation
	default:
		return unknownLabel
	}
}
