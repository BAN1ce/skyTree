---
topic: K8s capacity baseline for SkyTree and Scylla
audience: developer,sre,architect
doc_type: explanation,how-to
dependencies:
  - Kubernetes
  - ScyllaDB
  - SkyTree Makefile
keywords:
  - k8s
  - capacity
  - scylla
  - mqtt
  - beta
---

# K8s Capacity Baseline

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## Scope

This baseline applies only to the K8s deployment path launched from `skyTree/Makefile`:

- `make k8s-up`
- `make test-scylla-integration`
- `make test-distributed`
- `make beta-check`

It does not define requirements for the repository-level `start-skyTree-full.sh` developer script.

## Baseline Assumptions

- The sizing target is online MQTT device connections, not maximum publish throughput.
- Default traffic is light to medium: persistent connections, QoS0/QoS1 mix, payloads under 1KB, low-frequency telemetry, and 30% to 50% headroom.
- TLS/mTLS, QoS2, high publish frequency, large offline backlogs, retained-message fanout, and heavy shared subscription traffic require separate load testing.
- Production keepalive should use 60s to 120s. The Makefile K8s profile uses 60s to avoid local-test keepalive pressure at 10K+ connections.

## Default Makefile K8s Profile

The default Makefile K8s profile is sized as a 10K-connection beta baseline:

| Component | Replicas | CPU Request | Memory Request | Persistent Storage |
| --- | ---: | ---: | ---: | ---: |
| SkyTree | 3 | 1 vCPU each | 2Gi each | 20Gi each |
| Scylla | 3 | 1 vCPU each | 2Gi each | 20Gi each |

Minimum practical test-cluster capacity for this profile is about 8 vCPU and 20Gi memory before OS, CNI, gateway, Prometheus, Grafana, and storage overhead.

## Capacity Estimates

These estimates are starting points for planning and must be verified by real connection, publish, storage, and failover tests.

| Online Devices | SkyTree Baseline | Scylla Baseline | Notes |
| ---: | --- | --- | --- |
| 10K | 3 pods, 1 vCPU / 2Gi each | 3 pods, 1 vCPU / 2Gi each | Default beta K8s profile. |
| 100K | 3 pods, 4 vCPU / 8Gi each | 3 pods, 4 vCPU / 16Gi each | Recommended next profile for larger beta tests. |
| 1M | 8-12 pods, 8 vCPU / 16Gi each | 6-9 pods, 16 vCPU / 64Gi each | Requires backlog dashboards and hotspot checks. |
| 10M | 80-120 pods, 8-16 vCPU / 24-32Gi each | 24-60 pods, 16-32 vCPU / 128Gi each | Requires multi-AZ design, sharding, capacity SLOs, and sustained soak tests. |

## Scylla Notes

- The Makefile K8s profile uses a 3-node Scylla `StatefulSet`, not a single-node `Deployment`.
- The `skytree` keyspace uses replication factor 3 so `LOCAL_QUORUM` matches the backend topology.
- For production, prefer Scylla Operator or an existing managed Scylla/Cassandra-compatible service. The local manifests are a beta acceptance profile, not a full production install guide.

## Required Validation Before Capacity Claims

- Connection soak at the target online-device count.
- Publish throughput and latency tests at representative device rates.
- Offline backlog growth and drain tests.
- Shared subscription processing and rollback tests.
- Leader pod kill, broker pod kill, and node recovery tests.
- Scylla compaction, disk, and p99 CQL latency monitoring.

<!-- ai-generated-end -->
