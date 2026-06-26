---
topic: SkyTree runbook
audience: ai-agent,developer,sre
doc_type: how-to
dependencies:
  - Makefile
  - config yaml
keywords:
  - runbook
  - startup
  - troubleshooting
---

# SkyTree Runbook

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## 1. Local Start

### Prerequisites

- Go 1.24+
- accessible config file under `etc/`

### Start command

```bash
go run ./cmd/main.go --config ./etc/config.yaml
```

## 2. Common Validation Commands

```bash
make test-configs
make test-short
make test-race-core
```

## 3. Health and Observability

- HTTP health summary: `GET /health`
- Liveness probe: `GET /health/liveness`
- Readiness probe: `GET /health/readiness`
- Startup probe: `GET /health/startup`
- Metrics: `GET /metrics`
- Pprof: `GET /debug/pprof/*filepath`
- Cluster health: `GET /api/v1/cluster/health`
- Cluster overview: `GET /api/v1/cluster/overview`

Key Prometheus metrics use the `skytree_` prefix and bounded labels only.

Useful PromQL examples:

```promql
sum(rate(skytree_grpc_server_requests_total[5m])) by (service, method, result, code)
histogram_quantile(0.95, sum(rate(skytree_grpc_client_duration_seconds_bucket[5m])) by (le, service, method, to_node_id))
sum(rate(skytree_raft_requests_total[5m])) by (cluster, operation, result)
histogram_quantile(0.95, sum(rate(skytree_raft_request_duration_seconds_bucket[5m])) by (le, cluster, operation))
max(skytree_cluster_health_status) by (cluster, cluster_id)
sum(rate(skytree_delivery_send_attempts_total[5m])) by (path, qos, attempt)
```

## 4. TLS Bring-Up Checklist

For HTTP TLS:

- `server.tls.enabled=true`
- valid `server.tls.cert_file` and `server.tls.key_file`
- optional mTLS: set `server.tls.mtls_auth_mode` + `server.tls.ca_file`

For cluster gRPC TLS:

- `cluster.grpc.tls.enabled=true`
- valid cert/key (and CA if mTLS)
- if TLS disabled, explicitly set `cluster.grpc.allow_insecure=true`

## 5. Typical Failure Patterns

| Symptom | Likely root cause | First action |
| --- | --- | --- |
| startup exits early | config schema/validation failure | run `make test-configs` and inspect config field names |
| gRPC server fails to start | TLS required but not configured | check cluster TLS and allow_insecure settings |
| messages not delivered | task append/cursor drain issue | inspect delivery metadata/payload store config pairing |
| ACL API missing | ACL manager or credentials absent | verify ACL config and admin basic auth fields |
| cluster overview empty | overview provider unavailable | confirm health checker/cluster module initialization |

## 6. Operational Scripts

Useful scripts under `scripts/`:

- `validate_configs.sh`
- `test-build.sh`
- `start-k8s-console.sh`
- `test_scylla_integration.sh`
- `test_cluster_api.sh`
- `mqtt_functional_test.sh`
- `mqtt_stress_test.sh`

## 7. Safe Change Procedure

1. Update config/schema/contracts and tests first.
2. Run fast checks (`test-configs`, `test-short`).
3. For beta-blocking distributed or storage changes, run `make beta-check`.
4. Run race checks for broker core.
5. Validate management APIs and cluster path manually.
6. Update `docs/` docs affected by behavior changes.

<!-- ai-generated-end -->

<!-- PKB-metadata
last_updated: 2026-05-19
commit: 1cec350
updated_by: human+ai
doc_type: how-to
-->
