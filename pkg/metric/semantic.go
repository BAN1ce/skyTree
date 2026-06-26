package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OwnerTokenConflictsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_owner_token_conflicts_total",
		Help: "Total owner token fencing conflicts by bounded path and action.",
	}, []string{"path", "action"})

	CloseClientClientStageDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_close_client_client_stage_duration_seconds",
		Help:    "Duration of CloseClient client-side stages.",
		Buckets: defaultDurationBuckets,
	}, []string{"stage", "result"})

	CloseClientClientPathTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_close_client_client_path_total",
		Help: "Total CloseClient client-side path selections by bounded path and result.",
	}, []string{"path", "result"})

	CloseClientClientFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_close_client_client_failures_total",
		Help: "Total CloseClient client-side failures by bounded stage and reason.",
	}, []string{"stage", "reason"})

	CloseClientServerStageDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_close_client_server_stage_duration_seconds",
		Help:    "Duration of CloseClient server-side stages.",
		Buckets: defaultDurationBuckets,
	}, []string{"stage", "result"})

	RemoteCloseDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_remote_close_duration_seconds",
		Help:    "Duration of remote close client handling.",
		Buckets: defaultDurationBuckets,
	}, []string{"result"})

	RemoteCloseFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_remote_close_failures_total",
		Help: "Total remote close client failures by bounded reason.",
	}, []string{"reason"})

	SharedRollbackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_shared_rollback_total",
		Help: "Total shared subscription rollback decisions by bounded reason and result.",
	}, []string{"reason", "result"})

	ProcessingTimeoutTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_processing_timeout_total",
		Help: "Total processing timeout requeue decisions by bounded path and result.",
	}, []string{"path", "result"})

	DuplicateDeliveryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_duplicate_delivery_total",
		Help: "Total duplicate delivery prevention events by bounded path and stage.",
	}, []string{"path", "stage"})

	DeliveryCursorLagSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_delivery_cursor_lag_seconds",
		Help:    "Lag between delivery task timestamp and cursor progress or send observation.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 3, 5, 10, 30, 60, 300},
	}, []string{"path"})
)

func RecordOwnerTokenConflict(path, action string) {
	OwnerTokenConflictsTotal.WithLabelValues(
		normalizeOwnerTokenPath(path),
		normalizeOwnerTokenAction(action),
	).Inc()
}

func RecordCloseClientClientStage(stage, result string, duration time.Duration) {
	CloseClientClientStageDurationSeconds.WithLabelValues(
		normalizeCloseClientStage(stage),
		normalizeCloseClientStageResult(result),
	).Observe(durationSeconds(duration))
}

func RecordCloseClientClientPath(path, result string) {
	CloseClientClientPathTotal.WithLabelValues(
		normalizeCloseClientPath(path),
		normalizeCloseClientStageResult(result),
	).Inc()
}

func RecordCloseClientClientFailure(stage, reason string) {
	CloseClientClientFailuresTotal.WithLabelValues(
		normalizeCloseClientStage(stage),
		normalizeCloseClientFailureReason(reason),
	).Inc()
}

func RecordCloseClientServerStage(stage, result string, duration time.Duration) {
	CloseClientServerStageDurationSeconds.WithLabelValues(
		normalizeCloseClientStage(stage),
		normalizeCloseClientServerResult(result),
	).Observe(durationSeconds(duration))
}

func RecordRemoteClose(result string, duration time.Duration) {
	RemoteCloseDurationSeconds.WithLabelValues(normalizeRemoteCloseResult(result)).Observe(durationSeconds(duration))
}

func RecordRemoteCloseFailure(reason string) {
	RemoteCloseFailuresTotal.WithLabelValues(normalizeRemoteCloseFailure(reason)).Inc()
}

func RecordSharedRollback(reason, result string) {
	SharedRollbackTotal.WithLabelValues(
		normalizeSharedRollbackReason(reason),
		normalizeSemanticResult(result),
	).Inc()
}

func RecordProcessingTimeout(path, result string) {
	ProcessingTimeoutTotal.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeSemanticResult(result),
	).Inc()
}

func RecordDuplicateDelivery(path, stage string) {
	DuplicateDeliveryTotal.WithLabelValues(
		normalizeDeliveryPath(path),
		normalizeDuplicateDeliveryStage(stage),
	).Inc()
}

func RecordDeliveryCursorLag(path string, lag time.Duration) {
	DeliveryCursorLagSeconds.WithLabelValues(normalizeDeliveryPath(path)).Observe(durationSeconds(lag))
}

func normalizeOwnerTokenPath(path string) string {
	switch path {
	case "session", "subscription", "remote_close", "will":
		return path
	default:
		return unknownLabel
	}
}

func normalizeOwnerTokenAction(action string) string {
	switch action {
	case "ignore", "reject", "skip_close":
		return action
	default:
		return "reject"
	}
}

func normalizeCloseClientStage(stage string) string {
	switch stage {
	case "endpoint_lookup", "get_conn_total", "rpc_invoke", "response_validate", "read_client", "close_with_session_taken_over", "delete_client", "total":
		return stage
	default:
		return unknownLabel
	}
}

func normalizeCloseClientPath(path string) string {
	switch path {
	case "conn_cache_hit", "conn_cache_miss", "dial_new_conn", "drop_conn", "local_skip":
		return path
	default:
		return unknownLabel
	}
}

func normalizeCloseClientFailureReason(reason string) string {
	switch reason {
	case "deadline_exceeded", "canceled", "unavailable", "unknown", "not_found", "owner_conflict", "close_error", "connection_error", "unexpected_response":
		return reason
	default:
		return "unexpected_response"
	}
}

func normalizeCloseClientStageResult(result string) string {
	switch result {
	case "success", "timeout", "canceled", "error":
		return result
	default:
		return "error"
	}
}

func normalizeCloseClientServerResult(result string) string {
	switch result {
	case "success", "not_found", "owner_conflict", "error":
		return result
	default:
		return "error"
	}
}

func normalizeRemoteCloseResult(result string) string {
	switch result {
	case "success", "not_found", "owner_conflict", "error":
		return result
	default:
		return "error"
	}
}

func normalizeRemoteCloseFailure(reason string) string {
	switch reason {
	case "not_found", "owner_conflict", "close_error", "manager_missing":
		return reason
	default:
		return "manager_missing"
	}
}

func normalizeSharedRollbackReason(reason string) string {
	switch reason {
	case "client_offline", "processing_timeout", "duplicate_guard":
		return reason
	default:
		return "client_offline"
	}
}

func normalizeSemanticResult(result string) string {
	switch result {
	case "success", "skip", "error":
		return result
	default:
		return "error"
	}
}

func normalizeDuplicateDeliveryStage(stage string) string {
	switch stage {
	case "enqueue", "runner", "ack":
		return stage
	default:
		return "runner"
	}
}
