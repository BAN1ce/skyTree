package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventBusEnqueueTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_eventbus_enqueue_total",
		Help: "Total number of eventbus listener enqueue attempts.",
	}, []string{"result"})

	EventBusDropTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_eventbus_drop_total",
		Help: "Total number of eventbus dropped deliveries.",
	}, []string{"reason"})

	EventBusListenerQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "skytree_eventbus_listener_queue_depth",
		Help: "Current aggregate queue depth across all eventbus listeners.",
	})
)

func RecordEventBusEnqueue(result string) {
	EventBusEnqueueTotal.WithLabelValues(normalizeEventBusEnqueueResult(result)).Inc()
}

func RecordEventBusDrop(reason string) {
	EventBusDropTotal.WithLabelValues(normalizeEventBusDropReason(reason)).Inc()
}

func SetEventBusListenerQueueDepth(depth float64) {
	if depth < 0 {
		depth = 0
	}
	EventBusListenerQueueDepth.Set(depth)
}

func normalizeEventBusEnqueueResult(result string) string {
	switch result {
	case "queued", "queued_after_drop":
		return result
	default:
		return "queued"
	}
}

func normalizeEventBusDropReason(reason string) string {
	switch reason {
	case "queue_overflow_oldest", "queue_overflow_newest", "listener_deleted", "listener_stopped":
		return reason
	default:
		return "listener_stopped"
	}
}
