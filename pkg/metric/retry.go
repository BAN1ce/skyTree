package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PublishRetryTaskCurrent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "skytree_mqtt_publish_retry_tasks_current",
		Help: "Current number of scheduled MQTT publish retry tasks.",
	})

	ExecuteRetryTask = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_retry_execute_total",
		Help: "Total number of executed MQTT publish retry tasks.",
	})

	PublishRetryAction = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_retry_actions_total",
		Help: "Total MQTT publish retry actions.",
	}, []string{"action"})

	PublishRetryCreateFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_retry_create_failures_total",
		Help: "Total number of failed retry task creations",
	}, []string{"qos", "reason"})
)

const (
	PublishRetryActionCreate       = "create"
	PublishRetryActionDelete       = "delete"
	PublishRetryActionTimeout      = "timeout"
	PublishRetryActionExecute      = "execute"
	PublishRetryActionCreateFailed = "create_failed"
)

func RecordPublishRetryAction(action string) {
	PublishRetryAction.WithLabelValues(normalizePublishRetryAction(action)).Inc()
}

func RecordPublishRetryCreateFailed(qos byte, reason string) {
	PublishRetryCreateFailed.WithLabelValues(normalizeMQTTQoS(qos), normalizePublishRetryReason(reason)).Inc()
}

func normalizePublishRetryAction(action string) string {
	switch action {
	case PublishRetryActionCreate,
		PublishRetryActionDelete,
		PublishRetryActionTimeout,
		PublishRetryActionExecute,
		PublishRetryActionCreateFailed:
		return action
	default:
		return unknownLabel
	}
}

func normalizePublishRetryReason(reason string) string {
	switch reason {
	case "queue_full", "invalid_task", "schedule_error":
		return reason
	default:
		return unknownLabel
	}
}
