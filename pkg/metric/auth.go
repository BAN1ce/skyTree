package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AuthRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_auth_requests_total",
		Help: "Total number of AUTH requests",
	}, []string{"provider_type", "result"})

	AuthRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_mqtt_auth_request_duration_seconds",
		Help:    "Duration of AUTH requests in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
	}, []string{"provider_type"})

	AuthRequestFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_auth_request_failures_total",
		Help: "Total number of failed AUTH requests",
	}, []string{"provider_type", "error_type"})
)

func RecordAuthRequest(providerType string, result string) {
	AuthRequestTotal.WithLabelValues(providerType, result).Inc()
}

func RecordAuthDuration(providerType string, duration time.Duration) {
	AuthRequestDuration.WithLabelValues(providerType).Observe(durationSeconds(duration))
}

func RecordAuthRequestFailed(providerType string, errorType string) {
	AuthRequestFailed.WithLabelValues(providerType, errorType).Inc()
}
