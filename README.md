# SkyTree

SkyTree is a distributed [MQTT 5](https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.html) broker written in Go. A single process combines MQTT data-plane handling, subscription/session state centers, delivery queue and payload stores, management HTTP API, and inter-node gRPC notification.

> **Alpha preview** — APIs and behavior may change. Not recommended for production use. This release does not include the Web Console UI.

## Features

- MQTT 5 protocol (CONNECT, SUBSCRIBE, PUBLISH, QoS flows, session persistence)
- Single-node and clustered deployment modes
- Dragonboat-backed state centers for session, subscription, and will-delay in cluster mode
- HTTP management API, health probes, and Prometheus metrics
- Configurable delivery stores (Badger, Scylla, and others)

## Requirements

- Go 1.24+

## Quick Start

```bash
go run ./cmd/main.go --config ./etc/config.example.yaml
```

## Development

```bash
make test-configs   # validate example configs
make test-short     # unit tests (short mode)
make test-race-core # race detector on core packages
```

## Documentation

- [Overview](docs/00-overview.md) — architecture and runtime modes
- [Runbook](docs/06-runbook.md) — startup, health endpoints, troubleshooting
- [Testing](docs/07-testing.md) — test matrix and beta scenarios

## Runtime Modes

| Mode | `cluster.enable` | `local_state.enable` | Typical stores |
| --- | --- | --- | --- |
| In-memory single node | false | false | Badger/Redis, single-node delivery store |
| Durable single node | false | true | Local WAL state centers + single-node delivery store |
| Cluster | true | N/A | Dragonboat-backed centers + Scylla or cluster-compatible stores |

## License

MIT — see [LICENSE](LICENSE).

## Links

- Repository: https://github.com/BAN1ce/skyTree
- Release: [v0.1.0-alpha](https://github.com/BAN1ce/skyTree/releases/tag/v0.1.0-alpha)
