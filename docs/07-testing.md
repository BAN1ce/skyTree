---
topic: SkyTree testing strategy
audience: ai-agent,developer
doc_type: reference,how-to
dependencies:
  - Go test
  - Makefile
keywords:
  - testing
  - race
  - ci
---

# SkyTree Testing Guide

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## 1. Test Entry Commands

From `Makefile`:

- `make test`: run all tests.
- `make test-short`: short suite for fast iteration.
- `make test-race-core`: race-focused core paths.
- `make test-configs`: config schema/consistency checks.
- `make k8s-up`: build and start the local K8s SkyTree stack.
- `make k8s-status`: print local K8s stack status.
- `make k8s-down`: delete local K8s stack resources.
- `make test-distributed`: run Gardener beta cluster scenarios against the local K8s stack.
- `make test-scylla-integration`: run Scylla-backed delivery E2E against the K8s Scylla service.
- `make beta-check`: run short/race core tests plus distributed beta and Scylla integration.
- `make ci`: `test-configs + test-short + test-race-core`.
- `make test-mqtt5-compliance`: planned MQTT5 compliance suite entry. Until this target exists, maintain the matrix in `docs/changes/2026-06-23-mqtt5-compliance-matrix.md` with package-level verification commands.

The Makefile K8s stack uses a 3-node Scylla StatefulSet by default. The baseline resource profile targets 10K online MQTT connections:

- SkyTree: 3 pods, 1 vCPU and 2Gi memory each.
- Scylla: 3 pods, 1 vCPU and 2Gi memory each, 20Gi PVC each.
- Practical test-cluster capacity should be at least about 8 vCPU and 20Gi memory before system and observability overhead.

## 2. Test Layering

| Layer | Typical packages | Goal |
| --- | --- | --- |
| Unit tests | `config/*`, `api/*`, small storage modules | behavior correctness and validation logic |
| Core/race tests | `internal/broker/core`, delivery/session/sub/will modules, `pkg/cluster/raft` | concurrency safety and lifecycle robustness |
| Storage integration-like tests | `pkg/storage/delivery/*`, `pkg/storage/keystore/*` | backend adapter contract correctness |
| K8s distributed beta | `../gardener` cluster beta scenarios | leader pod kill, broker pod kill, recovery, fast reconnect, shared rollback |
| Scylla delivery E2E | `pkg/storage/delivery` with `integration,scylla` tags | real payload/task/cursor/dedupe/shared rollback behavior |

## 3. Must-Cover Scenarios For New Features

- Config load + validation fail-fast behavior.
- MQTT handler behavior changes (ACK semantics, invalid packet handling).
- Delivery persistence consistency:
  - payload saved and retrievable
  - task cursor progression and idempotence
- Cluster mode behavior does not regress single-node mode.
- Shutdown path does not leak goroutines/resources.

## 4. Recommended Local Test Matrix

1. `make test-configs`
2. `go test ./api/... ./config/...`
3. `make test-short`
4. `make test-race-core`

For risky infra changes:

- `make test`
- `make beta-check` before beta handoff.

WAL local state machine changes must include:

- `go test ./internal/localstate/walsm`
- a fault-injection test proving commit save failure does not apply memory state.

WAL commit failure semantics: `Write` validates update bytes, saves pending WAL, saves commit WAL, then applies the state machine update. If commit save fails, `Write` returns an error and memory state is not mutated. If `Update` fails after a durable commit, treat the engine as failed and restart/replay; callers must not blindly retry on the same engine.

## 5. Debugging Failed Tests

- Use `-run` to isolate suites.
- Add deterministic seed/time controls in flaky async tests.
- For race failures:
  - verify context cancellation path,
  - verify channel ownership and close semantics,
  - verify shared-state locking strategy.

## 6. AI Agent Testing Checklist

- Did the change include or adjust tests?
- Did config validation get updated for new fields?
- Did race-sensitive package tests pass?
- Did `make beta-check` pass for beta-blocking distributed/storage changes?
- Did behavior-level docs in `docs/` update alongside tests?

## 7. MQTT5 Compliance Matrix Maintenance

- Update `docs/changes/2026-06-23-mqtt5-compliance-matrix.md` when CONNECT, PUBLISH, SUBSCRIBE, AUTH, ACK, Retain, Will, or Shared Subscription behavior changes.
- A feature can be marked `Supported` only when it has a repeatable test or beta scenario entry.
- Keep matrix gaps focused on MQTT5 broker semantics. Deployment, dashboard, and capacity gaps belong in production-readiness docs.
- Do not add client IDs, topics, message IDs, owner tokens, payloads, or customer content to compliance logs or metrics.

<!-- ai-generated-end -->

<!-- PKB-metadata
last_updated: 2026-05-19
commit: 1cec350
updated_by: human+ai
doc_type: reference,how-to
-->
