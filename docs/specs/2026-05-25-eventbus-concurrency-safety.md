# EventBus Concurrency Safety Review

## Background

`pkg/eventbus.EventCenter` is used as the local notification bus for client delivery events. The main runtime paths are:

- Local delivery notify: `internal/broker/delivery/notify/client_delivery_event.go`
- Remote gRPC delivery notify: `internal/grpc/service_client_delivery_notify.go`
- Per-client listener registration: `internal/broker/core/client/delivery_runner.go`

The event name is client-scoped through `eventbus.ReceiveClientDeliveryEventName(clientID)`, so production usage naturally creates many different event names that may be accessed concurrently.

## Current Risk

The previous implementation used a single global map:

```go
type EventCenter struct {
	mux   *syncx.ShardedLock
	event map[string]*eventListeners
}
```

`syncx.ShardedLock` selects a lock by `eventName`, but every shard still protects the same `event` map. When two different event names map to different locks, operations such as `AddListener`, `Emit`, `DeleteListener`, and `EventListenerCount` can read and write the same Go map concurrently.

Observed failure mode:

- `go test -race ./pkg/eventbus` reports races on `EventCenter.event`.
- The race can escalate into `fatal error: concurrent map read and map write`.

There was also a delivery window in `internal/grpc/service_client_delivery_notify.go`:

```go
if s.localEvent.EventListenerCount(eventName) == 0 {
	continue
}
```

This check was not atomic with `Emit`. If a listener was added after the count check and before the skipped emit, the notification could be lost.

## Implemented Design

`EventCenter` now uses true sharded storage:

- `EventCenter` owns `[]eventShard`.
- Each `eventShard` owns its own `sync.RWMutex`.
- Each `eventShard` owns its own `map[string]*eventListeners`.
- `eventName` is hashed to a shard before touching any event state.

`Emit` now uses per-listener async queue workers:

1. Lock the selected shard.
2. Create or load the `eventListeners`.
3. Update `currentMeta`.
4. Copy the current listener runtime snapshot.
5. Unlock the shard.
6. Enqueue to each listener runtime without blocking the emitter:
   - queue has bounded capacity.
   - when full, drop oldest first, then enqueue newest.

Each listener runtime has an independent worker goroutine and serial queue, so one slow listener no longer blocks `Emit` or other listeners.

The gRPC delivery notify path now calls `Emit` directly. Empty listener sets are handled inside `Emit`, so there is no separate `EventListenerCount` decision window.

## Event Semantics

- `Emit` is asynchronous: the caller returns after enqueue attempts finish.
- Handler execution order is not guaranteed because handlers are stored in a map.
- Per-listener callback execution remains FIFO for that listener queue.
- A listener deleted during an in-flight `Emit` may still finish the callback currently running, but queued pending events are dropped.
- A listener added during an in-flight `Emit` does not receive that in-flight event.
- If a listener queue is full, oldest pending event is dropped and newest event is kept.
- `Emit` with no listener still records `currentMeta`.
- `AddListener` returns the latest `currentMeta` for that event name.
- `EventListenerCount` is an observation API only. It must not be used to decide whether to emit.

## Concurrency Test Checklist

- Different event names concurrently call `AddListener`, `Emit`, `EventListenerCount`, and `DeleteListener`.
- Same event name concurrently calls `AddListener`, `Emit`, and `DeleteListener`.
- Handler can re-enter `EventCenter` through `AddListener`, `DeleteListener`, and nested `Emit`.
- `DeleteListener` racing with an in-flight `Emit` keeps delivery well-defined while allowing pending events to be dropped.
- `Emit` with no listener records meta, and later `AddListener` receives that meta.
- gRPC notify path no longer skips `Emit` because of a stale `EventListenerCount`.
- `go test -race ./pkg/eventbus` passes without data races.

## Regression Risk Checklist

- Empty event entries are retained after the last listener is deleted so `currentMeta` can remain available. This changes memory behavior compared with deleting the event entry immediately.
- `currentMeta` may now be retained longer for event names that previously would have been removed after listener deletion.
- Handler execution is asynchronous per listener queue. A slow listener can no longer block the caller.
- Per-listener queue overflow may drop historical events under sustained overload (oldest-first).
- Handler order remains unstable because map iteration order is intentionally unspecified.
- `EventListenerCount` remains available but should only be used for diagnostics or metrics.
- The shard count is fixed at 256. Extremely skewed event names can still create a hot shard, but they cannot corrupt shared state.

## Verification

Required:

```bash
go test -race ./pkg/eventbus
go test ./internal/grpc ./pkg/eventbus
go test ./internal/grpc ./internal/broker/delivery/notify ./internal/broker/core/client
```

Recommended before merge:

```bash
go test ./...
```
