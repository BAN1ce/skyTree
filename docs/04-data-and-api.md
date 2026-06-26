---
topic: SkyTree data and api reference
audience: ai-agent,developer
doc_type: reference
dependencies:
  - Gin
  - gRPC
  - storage backends
keywords:
  - api
  - config
  - storage
  - data-model
---

# SkyTree Data And API

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## 1. Configuration Model

Root runtime config is `config.AppConfig`:

- `server`: HTTP API server and TLS.
- `broker`: listeners, MQTT limits, retain behavior, local state.
- `storage`: key/value and delivery storage settings.
- `cluster`: node membership, grpc settings, health check.
- `retry`, `delivery`, `scheduler`, `logging`, `plugins`.

Config load path:

1. YAML file read.
2. Environment overrides YAML.
3. `${ENV}` expansion.
4. Validation and normalized checks.

## 2. Delivery Store Data Plane / Metadata Plane

SkyTree splits delivery persistence into two planes:

- Metadata plane (`DeliveryQueueStore`): tasks, cursor, shared-task metadata.
- Payload plane (`MessagePayloadStore`): message body bytes.

`ResolveClientDeliverySpec()` enforces:

- queue and payload driver must both be set;
- supported pair only:
  - `single_node_badger + single_node_badger`
  - `scylla + scylla`
- single_node_badger queue is forbidden when `cluster.enable=true`.

## 3. HTTP API Surface

Base server: Gin with release mode.

### 3.1 Public/ops routes

- `GET /health`
- `GET /metrics` exposes Prometheus metrics with the `skytree_` prefix for MQTT, delivery, gRPC, Raft, store, eventbus, and cluster health signals.
- `GET /debug/pprof/*filepath`

### 3.2 ACL admin routes (`/api/v1/acl`)

Protected by basic auth and only enabled when ACL manager + admin credentials exist.

- `GET /ruleset`
- `PUT /ruleset`
- `DELETE /ruleset`
- `GET /rule?username=&client_id=`
- `PUT /rule`
- `DELETE /rule?username=&client_id=`

### 3.3 Cluster routes (`/api/v1/cluster`)

- `GET /health`
- `GET /health/:cluster_id`
- `GET /overview`

## 4. gRPC Surface

Cluster gRPC server provides:

- client-delivery notify service
- close-client service

TLS policy:

- if `cluster.grpc.tls.enabled=true`: cert/key required (optional mTLS).
- else only allowed when `cluster.grpc.allow_insecure=true`; otherwise startup fails.

## 5. Data Contracts Worth Tracking

| Contract | Why important |
| --- | --- |
| Session ownership token | Prevent stale node/client from overriding active owner state. |
| Delivery task cursor | Guarantees replay/resume behavior for connected/offline client. |
| Shared subscription task status | Supports claim, delivery, and rollback semantics. |
| Health status snapshot | Exposes cluster health trend and troubleshooting signal. |

## 6. Backward-Compatibility Rules For AI Changes

- Keep existing API path compatibility unless a migration plan is included.
- Any new config field must include YAML/env tag and validation path.
- Avoid direct use of raw map payloads for major data structures; keep typed structs.
- Never log secrets or tokens.

<!-- ai-generated-end -->

<!-- PKB-metadata
last_updated: 2026-05-19
commit: 1cec350
updated_by: human+ai
doc_type: reference
-->
