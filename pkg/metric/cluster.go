package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RaftRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_raft_requests_total",
		Help: "Total number of Raft client read/write requests.",
	}, []string{"cluster", "cluster_id", "operation", "result"})

	RaftRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_raft_request_duration_seconds",
		Help:    "Duration of Raft client read/write requests.",
		Buckets: defaultDurationBuckets,
	}, []string{"cluster", "cluster_id", "operation", "result"})

	RaftRequestBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_raft_request_bytes",
		Help:    "Payload size of Raft client write requests.",
		Buckets: defaultPayloadBuckets,
	}, []string{"cluster", "cluster_id", "operation"})

	RaftTimeoutsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_raft_timeouts_total",
		Help: "Total number of Raft client requests that timed out.",
	}, []string{"cluster", "cluster_id", "operation"})

	RaftGroupStarted = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_raft_group_started",
		Help: "Whether the local node started a Raft group.",
	}, []string{"cluster", "cluster_id"})

	RaftGroupLeader = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_raft_group_leader",
		Help: "Current Raft group leader by leader node ID label.",
	}, []string{"cluster", "cluster_id", "leader_node_id"})

	RaftGroupStartTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_raft_group_start_total",
		Help: "Total number of Raft group start attempts.",
	}, []string{"cluster", "cluster_id", "result"})

	ClusterHealthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_cluster_health_status",
		Help: "Cluster health status: 1 healthy, 0 unhealthy, -1 unknown.",
	}, []string{"cluster", "cluster_id"})

	ClusterHealthLatencySeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_cluster_health_latency_seconds",
		Help: "Last cluster health check latency in seconds.",
	}, []string{"cluster", "cluster_id"})
)

func RecordRaftRequest(
	cluster string,
	clusterID uint64,
	operation string,
	err error,
	duration time.Duration,
	requestBytes int,
) {
	clusterIDLabel := uint64Label(clusterID)
	result := resultFromError(err)
	RaftRequestsTotal.WithLabelValues(cluster, clusterIDLabel, normalizeRaftOperation(operation), result).Inc()
	RaftRequestDurationSeconds.WithLabelValues(cluster, clusterIDLabel, normalizeRaftOperation(operation), result).
		Observe(durationSeconds(duration))
	RaftRequestBytes.WithLabelValues(cluster, clusterIDLabel, normalizeRaftOperation(operation)).Observe(bytesValue(requestBytes))
	if result == "timeout" {
		RaftTimeoutsTotal.WithLabelValues(cluster, clusterIDLabel, normalizeRaftOperation(operation)).Inc()
	}
}

func RecordRaftGroupStart(cluster string, clusterID uint64, err error) {
	clusterIDLabel := uint64Label(clusterID)
	result := resultFromError(err)
	RaftGroupStartTotal.WithLabelValues(cluster, clusterIDLabel, result).Inc()
	if err == nil {
		RaftGroupStarted.WithLabelValues(cluster, clusterIDLabel).Set(1)
	}
}

func SetRaftGroupStarted(cluster string, clusterID uint64, started bool) {
	value := 0.0
	if started {
		value = 1
	}
	RaftGroupStarted.WithLabelValues(cluster, uint64Label(clusterID)).Set(value)
}

func SetRaftGroupLeader(cluster string, clusterID uint64, leaderNodeID uint64) {
	RaftGroupLeader.WithLabelValues(cluster, uint64Label(clusterID), uint64Label(leaderNodeID)).Set(1)
}

func RecordClusterHealth(cluster string, clusterID uint64, status float64, latency time.Duration) {
	clusterIDLabel := uint64Label(clusterID)
	ClusterHealthStatus.WithLabelValues(cluster, clusterIDLabel).Set(status)
	ClusterHealthLatencySeconds.WithLabelValues(cluster, clusterIDLabel).Set(durationSeconds(latency))
}

func normalizeRaftOperation(operation string) string {
	switch operation {
	case "read", "write":
		return operation
	default:
		return unknownLabel
	}
}
