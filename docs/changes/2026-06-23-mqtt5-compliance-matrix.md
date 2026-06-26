---
topic: MQTT5 compliance matrix
audience: developer,architect,sre
doc_type: reference
dependencies:
  - Go 1.24
  - MQTT 5.0
keywords:
  - mqtt5
  - compliance
  - broker
  - beta
---

# SkyTree MQTT5 Compliance Matrix

Date: 2026-06-23  
Scope: Beta post-enhancement declaration for core MQTT5 broker behavior.  
Status: Phase 2 baseline.

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## Support Levels

| Level | Meaning |
| --- | --- |
| Supported | Implemented and covered by focused tests or beta scenarios. |
| Partial | Core behavior exists, but at least one spec option, production policy, or end-to-end verification is missing. |
| Not supported | Intentionally unavailable or not implemented. |
| Not verified | Implementation may exist, but no reliable verification entry is identified yet. |

## Feature Matrix

| Area | Support | Implemented behavior | Known gaps | Verification |
| --- | --- | --- | --- | --- |
| CONNECT | Partial | Protocol validation, server-assigned ClientID, Clean Start, Session Expiry, Server Keep Alive, Receive Maximum, Maximum Packet Size, owner token fencing, session takeover, mTLS certificate context, and initial Enhanced AUTH handoff are implemented in `internal/broker/client`. | Production authentication policy is plugin-driven and still needs deployment templates. Cross-node takeover is covered by beta scenarios but should remain part of release gating. | `go test ./internal/broker/client`, `go test ./internal/broker/core`, `make test-distributed`. |
| PUBLISH | Partial | Topic Alias, Payload Format Indicator validation, QoS0/QoS1/QoS2 publish paths, Receive Maximum enforcement, No Matching Subscribers, Retain Available, Maximum QoS handling, duplicate QoS2 first-phase protection, and persistent delivery routing are implemented. | Large-scale duplicate delivery and cursor lag need semantic metrics and soak tests. Topic/payload policy remains plugin/config dependent. | `go test ./internal/broker/client`, `go test ./internal/broker/core`, `go test ./pkg/storage/delivery`, `make test-scylla-integration`. |
| SUBSCRIBE | Partial | Shared subscriptions, Subscription Identifier, No Local, Retain As Published, Retain Handling, topic filter validation, and shared subscription capability flags are implemented. | Multi-node shared group failover needs continuous beta validation. Compliance case mapping for invalid filters and mixed options should be expanded. | `go test ./internal/broker/client`, `go test ./internal/broker/subcenter/...`, `make test-distributed`. |
| AUTH | Partial | Enhanced AUTH provider hooks and AUTH packet mapping are implemented through broker auth/plugin packages. CONNECT can pause for multi-step AUTH. | Production-ready provider examples and negative-path compliance cases are incomplete. Auth secrets must remain external to config and logs. | `go test ./internal/broker/auth`, `go test ./internal/broker/client -run Auth`. |
| ACK | Supported | PUBACK, PUBREC, PUBREL, and PUBCOMP flows are implemented for inbound and outbound QoS1/QoS2. Negative ACK reason codes complete terminal flows where applicable. Persistent outbound cursor advancement waits for terminal ACK. | More interop tests against third-party MQTT5 clients are recommended. | `go test ./internal/broker/client -run 'PubAck|PubRec|PubRel|PubComp|QoS2|DeliveryRunner'`. |
| Retain | Supported | Retained message store, retained publish delivery, retain expiry cleanup, Retain As Published handling, and retained QoS2 commit timing are implemented. | High-cardinality retain topic soak tests are still needed for capacity planning. | `go test ./internal/broker/retain`, `go test ./internal/broker/core -run Retain`, `go test ./internal/broker/client -run Retain`. |
| Will | Partial | Will Delay task creation, cleanup, expiry validation, and owner token protection are implemented through `internal/broker/willdelay` and session state. | Cross-node reconnect and delayed close races need to remain in beta distributed scenarios. | `go test ./internal/broker/willdelay/...`, `go test ./internal/broker/core -run Will`, `make test-distributed`. |
| Shared Subscription | Partial | Shared group routing, consumer leadership, pending/processing/completed task states, client offline rollback, processing timeout requeue, and shared client task cleanup are implemented. | Failover and rollback semantics need semantic metrics and ongoing distributed fault-injection. | `go test ./internal/broker/sharedsubscription/...`, `go test ./internal/broker/core -run Shared`, `make test-distributed`, `make test-scylla-integration`. |

## Compliance Gaps

| Gap | Impact | Planned follow-up |
| --- | --- | --- |
| Spec-case mapping is not exhaustive. | It is hard to claim exact MQTT5 section-level compliance. | Add a case ID column when a dedicated `make test-mqtt5-compliance` suite exists. |
| Third-party client interop is not automated. | Broker behavior may pass internal tests but fail client-specific edge cases. | Add an interop smoke suite with common MQTT5 clients. |
| Production auth/ACL policy is deployment-specific. | AUTH and CONNECT support cannot be declared as fully production-ready without templates. | Provide mTLS, username/password, ACL, and Enhanced AUTH examples. |
| Distributed shared subscription behavior depends on beta scenarios. | Shared rollback or failover regressions may not appear in unit tests. | Keep `make test-distributed` and semantic metrics in release gating. |

## Maintenance Rules

- Update this matrix whenever MQTT packet handling, delivery semantics, retain/will behavior, shared subscription behavior, or auth plugins change.
- Do not mark a feature `Supported` unless a repeatable verification entry exists.
- Keep gaps specific to MQTT5 behavior. Cluster membership, dashboard UI, and deployment capacity belong in production-readiness docs.
- Do not add direct identifiers, tokens, topics, payloads, or customer content to compliance logs or metrics.

<!-- ai-generated-end -->
