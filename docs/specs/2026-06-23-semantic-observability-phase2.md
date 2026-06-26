# Phase 2 Semantic Observability

## Summary

This spec adds semantic Prometheus metrics for MQTT5 beta operations that are hard to diagnose through component-level health alone: owner token fencing, remote client close, shared rollback, processing timeout, duplicate delivery prevention, and delivery cursor lag.

All metrics use bounded labels only. Do not add `client_id`, `topic`, `message_id`, `share_group`, owner token, payload, or customer content to metric labels.

## Metrics

### `skytree_owner_token_conflicts_total`

- Type: Counter
- Labels: `path`, `action`
- Label values:
  - `path`: `session`, `subscription`, `remote_close`, `will`, `unknown`
  - `action`: `ignore`, `reject`, `skip_close`
- Meaning: owner token fencing prevented a stale owner or stale request from mutating the active session.
- Instrumentation:
  - `internal/grpc/service_close_client.go`
  - `internal/broker/core/broker_keepalive.go`

### `skytree_remote_close_duration_seconds`

- Type: Histogram
- Labels: `result`
- Label values: `success`, `not_found`, `owner_conflict`, `error`
- Meaning: server-side duration of remote close handling.
- Instrumentation: `internal/grpc/service_close_client.go`

### `skytree_remote_close_failures_total`

- Type: Counter
- Labels: `reason`
- Label values: `not_found`, `owner_conflict`, `close_error`, `manager_missing`
- Meaning: remote close requests that did not close an active matching client.
- Instrumentation: `internal/grpc/service_close_client.go`

### `skytree_shared_rollback_total`

- Type: Counter
- Labels: `reason`, `result`
- Label values:
  - `reason`: `client_offline`, `processing_timeout`, `duplicate_guard`
  - `result`: `success`, `skip`, `error`
- Meaning: shared subscription rollback and requeue decisions.
- Instrumentation: `internal/broker/sharedsubscription/domain/rollback_service.go`

### `skytree_processing_timeout_total`

- Type: Counter
- Labels: `path`, `result`
- Label values:
  - `path`: `normal`, `shared`
  - `result`: `success`, `skip`, `error`
- Meaning: tasks detected as processing-timeout and requeued or skipped.
- Instrumentation: `internal/broker/sharedsubscription/domain/rollback_service.go`

### `skytree_duplicate_delivery_total`

- Type: Counter
- Labels: `path`, `stage`
- Label values:
  - `path`: `normal`, `shared`
  - `stage`: `enqueue`, `runner`, `ack`
- Meaning: duplicate delivery prevention events across enqueue, runner, and ACK stages.
- Instrumentation:
  - `internal/broker/core/broker_delivery_normal.go`
  - `internal/broker/sharedsubscription/consumer/consumer.go`
  - `internal/broker/client/delivery_runner.go`
  - `internal/broker/client/client_handler_qos_flow.go`

### `skytree_delivery_cursor_lag_seconds`

- Type: Histogram
- Labels: `path`
- Label values: `normal`, `shared`
- Meaning: `time.Now() - delivery_task.TS` when first send or cursor advancement is observed.
- Instrumentation: `internal/broker/client/delivery_runner.go`, `internal/broker/client/client_handler_qos_flow.go`

## PromQL Examples

Owner token conflict rate:

```promql
sum(rate(skytree_owner_token_conflicts_total[5m])) by (path, action)
```

Remote close p95 latency:

```promql
histogram_quantile(0.95, sum(rate(skytree_remote_close_duration_seconds_bucket[5m])) by (le, result))
```

Remote close failure rate:

```promql
sum(rate(skytree_remote_close_failures_total[5m])) by (reason)
```

Shared rollback rate:

```promql
sum(rate(skytree_shared_rollback_total[5m])) by (reason, result)
```

Processing timeout rate:

```promql
sum(rate(skytree_processing_timeout_total[5m])) by (path, result)
```

Duplicate delivery prevention rate:

```promql
sum(rate(skytree_duplicate_delivery_total[5m])) by (path, stage)
```

Delivery cursor lag p95:

```promql
histogram_quantile(0.95, sum(rate(skytree_delivery_cursor_lag_seconds_bucket[5m])) by (le, path))
```

## Alert Threshold Draft

- Owner token conflicts:
  - Warning: sustained non-zero rate for 15m.
  - Critical: sudden spike above historical baseline during reconnect storms.
- Remote close:
  - Warning: p95 > 0.3s for 10m or failure rate > 1% for 10m.
  - Critical: p95 > 1s for 10m or owner conflict spike after deploy.
- Shared rollback:
  - Warning: processing timeout rollback rate > 0 for 10m.
  - Critical: rollback error rate > 0 for 5m.
- Duplicate delivery:
  - Warning: duplicate prevention rate > 1% of delivery enqueue rate for 15m.
  - Critical: duplicate ACK or runner events spike after delivery changes.
- Delivery cursor lag:
  - Warning: p95 > 3s for 10m.
  - Critical: p95 > 10s for 10m.

## Validation

Focused tests:

```bash
go test ./pkg/metric
go test ./internal/grpc
go test ./internal/broker/sharedsubscription/domain
go test ./internal/broker/client -run 'TestDeliveryRunner|TestHandlePubAck|TestHandlePubComp'
```

Runtime check:

```bash
curl -s http://127.0.0.1:<port>/metrics | grep 'skytree_.*\\(owner_token\\|remote_close\\|shared_rollback\\|processing_timeout\\|duplicate_delivery\\|delivery_cursor_lag\\)'
```
