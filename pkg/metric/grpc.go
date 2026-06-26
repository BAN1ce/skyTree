package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
)

var (
	GRPCClientRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_grpc_client_requests_total",
		Help: "Total number of outgoing SkyTree gRPC requests.",
	}, []string{"service", "method", "to_node_id", "result", "code"})

	GRPCClientDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_grpc_client_duration_seconds",
		Help:    "Duration of outgoing SkyTree gRPC requests.",
		Buckets: defaultDurationBuckets,
	}, []string{"service", "method", "to_node_id", "result"})

	GRPCClientInflight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_grpc_client_inflight",
		Help: "Current in-flight outgoing SkyTree gRPC requests.",
	}, []string{"service", "method", "to_node_id"})

	GRPCClientPayloadBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_grpc_client_payload_bytes",
		Help:    "Payload size for outgoing SkyTree gRPC requests and responses.",
		Buckets: defaultPayloadBuckets,
	}, []string{"service", "method", "direction"})

	GRPCClientConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_grpc_client_connections",
		Help: "Current gRPC client connection state by target node.",
	}, []string{"to_node_id", "state"})

	GRPCServerRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_grpc_server_requests_total",
		Help: "Total number of incoming SkyTree gRPC requests.",
	}, []string{"service", "method", "result", "code"})

	GRPCServerDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_grpc_server_duration_seconds",
		Help:    "Duration of incoming SkyTree gRPC requests.",
		Buckets: defaultDurationBuckets,
	}, []string{"service", "method", "result"})

	GRPCServerInflight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "skytree_grpc_server_inflight",
		Help: "Current in-flight incoming SkyTree gRPC requests.",
	}, []string{"service", "method"})

	GRPCServerPayloadBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "skytree_grpc_server_payload_bytes",
		Help:    "Payload size for incoming SkyTree gRPC requests and responses.",
		Buckets: defaultPayloadBuckets,
	}, []string{"service", "method", "direction"})
)

func BeginGRPCClientRequest(service, method string, toNodeID uint64) func() {
	nodeID := uint64Label(toNodeID)
	GRPCClientInflight.WithLabelValues(service, method, nodeID).Inc()
	return func() {
		GRPCClientInflight.WithLabelValues(service, method, nodeID).Dec()
	}
}

func RecordGRPCClientRequest(
	service string,
	method string,
	toNodeID uint64,
	code codes.Code,
	err error,
	duration time.Duration,
	requestBytes int,
	responseBytes int,
) {
	result := resultFromGRPCCode(code, err)
	nodeID := uint64Label(toNodeID)
	GRPCClientRequestsTotal.WithLabelValues(service, method, nodeID, result, code.String()).Inc()
	GRPCClientDurationSeconds.WithLabelValues(service, method, nodeID, result).Observe(durationSeconds(duration))
	GRPCClientPayloadBytes.WithLabelValues(service, method, "request").Observe(bytesValue(requestBytes))
	GRPCClientPayloadBytes.WithLabelValues(service, method, "response").Observe(bytesValue(responseBytes))
}

func SetGRPCClientConnection(toNodeID uint64, state string, value float64) {
	switch state {
	case "connected", "error":
	default:
		state = unknownLabel
	}
	if value < 0 {
		value = 0
	}
	GRPCClientConnections.WithLabelValues(uint64Label(toNodeID), state).Set(value)
}

func BeginGRPCServerRequest(service, method string) func() {
	GRPCServerInflight.WithLabelValues(service, method).Inc()
	return func() {
		GRPCServerInflight.WithLabelValues(service, method).Dec()
	}
}

func RecordGRPCServerRequest(
	service string,
	method string,
	code codes.Code,
	err error,
	duration time.Duration,
	requestBytes int,
	responseBytes int,
) {
	result := resultFromGRPCCode(code, err)
	GRPCServerRequestsTotal.WithLabelValues(service, method, result, code.String()).Inc()
	GRPCServerDurationSeconds.WithLabelValues(service, method, result).Observe(durationSeconds(duration))
	GRPCServerPayloadBytes.WithLabelValues(service, method, "request").Observe(bytesValue(requestBytes))
	GRPCServerPayloadBytes.WithLabelValues(service, method, "response").Observe(bytesValue(responseBytes))
}
