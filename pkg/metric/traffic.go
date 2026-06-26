package metric

import (
	"time"

	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	NodeTrafficStateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_node_traffic_state",
		Help: "Node traffic readiness state as one-hot gauges.",
	}, []string{"node_id", "state"})

	NodeActivationAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_node_activation_attempts_total",
		Help: "Total node activation attempts.",
	}, []string{"node_id"})

	NodeActivationFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_node_activation_failures_total",
		Help: "Total node activation failures by bounded reason.",
	}, []string{"node_id", "reason"})

	NodeActivationDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_node_activation_duration_seconds",
		Help:    "Duration of node activation attempts.",
		Buckets: defaultDurationBuckets,
	}, []string{"node_id", "result"})

	PeerConnStateTransitionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_peer_conn_state_transitions_total",
		Help: "Total peer gRPC connection state transitions.",
	}, []string{"node_id", "from", "to"})

	PeerConnEvictionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_peer_conn_evictions_total",
		Help: "Total peer gRPC connection evictions by bounded reason.",
	}, []string{"node_id", "reason"})

	PeerPreflightDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_peer_preflight_duration_seconds",
		Help:    "Duration of peer gRPC preflight probes.",
		Buckets: defaultDurationBuckets,
	}, []string{"node_id", "result"})

	PeerRPCFastFailTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_peer_rpc_fast_fail_total",
		Help: "Total peer RPC fast-fail decisions by bounded reason.",
	}, []string{"node_id", "reason"})

	NodeMQTTReadinessFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_node_mqtt_readiness_failures_total",
		Help: "Total MQTT readiness probe failures by bounded reason.",
	}, []string{"node_id", "reason"})

	NodeMQTTReadyState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_node_mqtt_ready_state",
		Help: "Whether node MQTT ingress is ready (1) or not (0).",
	}, []string{"node_id"})
)

func SetNodeTrafficState(nodeID uint64, state cluster.TrafficState) {
	nodeIDLabel := uint64Label(nodeID)
	for _, candidate := range []cluster.TrafficState{
		cluster.TrafficStateUnknown,
		cluster.TrafficStateJoining,
		cluster.TrafficStateWarming,
		cluster.TrafficStateReady,
		cluster.TrafficStateSuspect,
		cluster.TrafficStateDraining,
	} {
		value := 0.0
		if normalizeTrafficStateLabel(candidate) == normalizeTrafficStateLabel(state) {
			value = 1
		}
		NodeTrafficStateGauge.WithLabelValues(nodeIDLabel, normalizeTrafficStateLabel(candidate)).Set(value)
	}
}

func RecordNodeActivationAttempt(nodeID uint64) {
	NodeActivationAttemptsTotal.WithLabelValues(uint64Label(nodeID)).Inc()
}

func RecordNodeActivationFailure(nodeID uint64, reason string) {
	NodeActivationFailuresTotal.WithLabelValues(uint64Label(nodeID), normalizeActivationFailureReason(reason)).Inc()
}

func RecordNodeActivationDuration(nodeID uint64, result string, duration time.Duration) {
	NodeActivationDurationSeconds.WithLabelValues(uint64Label(nodeID), normalizeActivationResult(result)).Observe(durationSeconds(duration))
}

func RecordPeerConnStateTransition(nodeID uint64, from, to string) {
	fromLabel := normalizePeerConnState(from)
	toLabel := normalizePeerConnState(to)
	if fromLabel == toLabel {
		return
	}
	PeerConnStateTransitionsTotal.WithLabelValues(uint64Label(nodeID), fromLabel, toLabel).Inc()
}

func RecordPeerConnEviction(nodeID uint64, reason string) {
	PeerConnEvictionsTotal.WithLabelValues(uint64Label(nodeID), normalizePeerConnEvictionReason(reason)).Inc()
}

func RecordPeerPreflight(nodeID uint64, result string, duration time.Duration) {
	PeerPreflightDurationSeconds.WithLabelValues(uint64Label(nodeID), normalizePeerPreflightResult(result)).Observe(durationSeconds(duration))
}

func RecordPeerRPCFastFail(nodeID uint64, reason string) {
	PeerRPCFastFailTotal.WithLabelValues(uint64Label(nodeID), normalizePeerFastFailReason(reason)).Inc()
}

func RecordNodeMQTTReadinessFailure(nodeID uint64, reason string) {
	NodeMQTTReadinessFailuresTotal.WithLabelValues(uint64Label(nodeID), normalizeActivationFailureReason(reason)).Inc()
	NodeMQTTReadyState.WithLabelValues(uint64Label(nodeID)).Set(0)
}

func SetNodeMQTTReady(nodeID uint64, ready bool) {
	value := 0.0
	if ready {
		value = 1
	}
	NodeMQTTReadyState.WithLabelValues(uint64Label(nodeID)).Set(value)
}

func normalizeTrafficStateLabel(state cluster.TrafficState) string {
	switch state {
	case cluster.TrafficStateJoining,
		cluster.TrafficStateWarming,
		cluster.TrafficStateReady,
		cluster.TrafficStateSuspect,
		cluster.TrafficStateDraining:
		return string(state)
	default:
		return string(cluster.TrafficStateUnknown)
	}
}

func normalizeActivationFailureReason(reason string) string {
	switch reason {
	case "grpc_probe_failed", "health_probe_failed", "mqtt_probe_failed", "timeout", "activation_pending":
		return reason
	default:
		return unknownLabel
	}
}

func normalizeActivationResult(result string) string {
	switch result {
	case "success", "timeout", "error":
		return result
	default:
		return "error"
	}
}

func normalizePeerConnState(state string) string {
	switch state {
	case "idle", "connecting", "ready", "transient_failure", "shutdown":
		return state
	default:
		return unknownLabel
	}
}

func normalizePeerConnEvictionReason(reason string) string {
	switch reason {
	case "transient_failure", "shutdown", "connecting_timeout", "rpc_unavailable", "rpc_timeout", "rpc_error", "node_not_ready":
		return reason
	default:
		return unknownLabel
	}
}

func normalizePeerPreflightResult(result string) string {
	switch result {
	case "success", "timeout", "error":
		return result
	default:
		return "error"
	}
}

func normalizePeerFastFailReason(reason string) string {
	switch reason {
	case "node_not_ready", "node_suspect", "node_draining":
		return reason
	default:
		return unknownLabel
	}
}
