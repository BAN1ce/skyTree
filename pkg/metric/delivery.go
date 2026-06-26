package metric

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DeliveryEnqueueDelaySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_delivery_enqueue_delay_seconds",
		Help:    "Duration of a single AppendClientTask call in the persistent delivery path.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 3},
	}, []string{"path", "qos", "result"})

	DeliveryWakeDelaySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_delivery_wake_delay_seconds",
		Help:    "Duration of delivery wake actions.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1, 3},
	}, []string{"path", "mode", "result"})

	DeliveryFirstSendDelaySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_delivery_first_send_delay_seconds",
		Help:    "Delay from delivery task timestamp to the first successful send attempt.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 1.5, 3, 5, 10},
	}, []string{"path", "qos"})

	DeliverySendAttemptTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_delivery_send_attempts_total",
		Help: "Total delivery send attempts, split by initial sends and retransmissions.",
	}, []string{"path", "qos", "attempt"})

	DeliveryAckDelaySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_delivery_ack_delay_seconds",
		Help:    "Delay from first downlink publish time to terminal acknowledgement.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 3, 5, 10, 12, 30},
	}, []string{"path", "qos", "result"})
)

func RecordDeliveryEnqueueDelay(path string, qos int, result string, duration time.Duration) {
	DeliveryEnqueueDelaySeconds.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDeliveryQoS(qos),
		normalizeDeliveryResult(result),
	).Observe(durationSeconds(duration))
}

func RecordDeliveryWakeDelay(path string, mode string, result string, duration time.Duration) {
	DeliveryWakeDelaySeconds.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDeliveryWakeMode(mode),
		normalizeDeliveryResult(result),
	).Observe(durationSeconds(duration))
}

func RecordDeliveryFirstSendDelay(path string, qos int, delay time.Duration) {
	DeliveryFirstSendDelaySeconds.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDeliveryQoS(qos),
	).Observe(durationSeconds(delay))
}

func RecordDeliverySendAttempt(path string, qos int, attempt string) {
	DeliverySendAttemptTotal.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDeliveryQoS(qos),
		normalizeDeliveryAttempt(attempt),
	).Inc()
}

func RecordDeliveryAckDelay(path string, qos int, result string, delay time.Duration) {
	DeliveryAckDelaySeconds.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDeliveryQoS(qos),
		normalizeDeliveryResult(result),
	).Observe(durationSeconds(delay))
}

func normalizeDeliveryPath(path string) string {
	switch path {
	case "normal", "shared":
		return path
	default:
		return "normal"
	}
}

func normalizeDeliveryQoS(qos int) string {
	switch qos {
	case 0, 1, 2:
		return strconv.Itoa(qos)
	default:
		return "0"
	}
}

func normalizeDeliveryResult(result string) string {
	switch result {
	case "success", "duplicate", "error", "negative":
		return result
	default:
		return "error"
	}
}

func normalizeDeliveryWakeMode(mode string) string {
	switch mode {
	case "event_local", "event_remote", "direct":
		return mode
	default:
		return "direct"
	}
}

func normalizeDeliveryAttempt(attempt string) string {
	switch attempt {
	case "initial", "retransmit":
		return attempt
	default:
		return "initial"
	}
}
