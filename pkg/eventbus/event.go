package eventbus

import (
	"hash/fnv"
	"sync"
	"sync/atomic"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/google/uuid"
)

const defaultShardCount = 256
const defaultListenerQueueCapacity = 64

type EventHandler[T any] func(T)

type eventCenterOptions struct {
	listenerQueueCapacity int
}

type EventCenterOption func(*eventCenterOptions)

func WithListenerQueueCapacity(capacity int) EventCenterOption {
	return func(options *eventCenterOptions) {
		if options == nil || capacity <= 0 {
			return
		}
		options.listenerQueueCapacity = capacity
	}
}

type listenerInvocation[T any] struct {
	payload T
}

type listenerRuntime[T any] struct {
	handler  EventHandler[T]
	queue    chan listenerInvocation[T]
	stopCh   chan struct{}
	stopOnce sync.Once
	stopped  atomic.Bool
	mu       sync.Mutex
	center   *EventCenter[T]
}

func newListenerRuntime[T any](handler EventHandler[T], queueCapacity int, center *EventCenter[T]) *listenerRuntime[T] {
	runtime := &listenerRuntime[T]{
		handler: handler,
		queue:   make(chan listenerInvocation[T], queueCapacity),
		stopCh:  make(chan struct{}),
		center:  center,
	}
	go runtime.run()
	return runtime
}

func (r *listenerRuntime[T]) run() {
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		select {
		case <-r.stopCh:
			return
		case invocation := <-r.queue:
			if r.center != nil {
				r.center.adjustQueueDepth(-1)
			}
			r.handler(invocation.payload)
		}
	}
}

func (r *listenerRuntime[T]) stop() {
	if r == nil {
		return
	}

	r.stopOnce.Do(func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.stopped.Store(true)
		close(r.stopCh)

		dropped := 0
		for {
			select {
			case <-r.queue:
				dropped++
			default:
				if dropped > 0 {
					if r.center != nil {
						r.center.adjustQueueDepth(-int64(dropped))
					}
					metric.RecordEventBusDrop("listener_deleted")
				}
				return
			}
		}
	})
}

func (r *listenerRuntime[T]) enqueue(payload T) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped.Load() {
		metric.RecordEventBusDrop("listener_stopped")
		return
	}

	select {
	case r.queue <- listenerInvocation[T]{payload: payload}:
		if r.center != nil {
			r.center.adjustQueueDepth(1)
		}
		metric.RecordEventBusEnqueue("queued")
		return
	default:
	}

	// Queue full: drop the oldest event first, then enqueue the newest one.
	select {
	case <-r.queue:
		if r.center != nil {
			r.center.adjustQueueDepth(-1)
		}
		metric.RecordEventBusDrop("queue_overflow_oldest")
	default:
	}

	if r.stopped.Load() {
		metric.RecordEventBusDrop("listener_stopped")
		return
	}

	select {
	case r.queue <- listenerInvocation[T]{payload: payload}:
		if r.center != nil {
			r.center.adjustQueueDepth(1)
		}
		metric.RecordEventBusEnqueue("queued_after_drop")
	default:
		metric.RecordEventBusDrop("queue_overflow_newest")
	}
}

type eventListeners[T any] struct {
	listeners   map[string]*listenerRuntime[T]
	currentMeta string
}

type eventShard[T any] struct {
	mu     sync.RWMutex
	events map[string]*eventListeners[T]
}

type EventCenter[T any] struct {
	shards                []eventShard[T]
	listenerQueueCapacity int
	queueDepth            atomic.Int64
}

func NewEventCenter[T any](opts ...EventCenterOption) *EventCenter[T] {
	options := eventCenterOptions{listenerQueueCapacity: defaultListenerQueueCapacity}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.listenerQueueCapacity <= 0 {
		options.listenerQueueCapacity = defaultListenerQueueCapacity
	}

	center := &EventCenter[T]{
		shards:                make([]eventShard[T], defaultShardCount),
		listenerQueueCapacity: options.listenerQueueCapacity,
	}
	for i := range center.shards {
		center.shards[i].events = make(map[string]*eventListeners[T])
	}
	metric.SetEventBusListenerQueueDepth(0)
	return center
}

func (e *EventCenter[T]) shardFor(eventName string) *eventShard[T] {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(eventName))
	return &e.shards[int(hash.Sum32()%uint32(len(e.shards)))]
}

func (e *EventCenter[T]) AddListener(eventName string, handler EventHandler[T]) (listenerId, meta string) {
	shard := e.shardFor(eventName)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	listen := shard.events[eventName]
	if listen == nil {
		listen = &eventListeners[T]{listeners: make(map[string]*listenerRuntime[T])}
		shard.events[eventName] = listen
	}

	listenerId = uuid.NewString()
	listen.listeners[listenerId] = newListenerRuntime(handler, e.listenerQueueCapacity, e)
	meta = listen.currentMeta
	logger.Logger.Debug().Str("eventName", eventName).Msg("add new event")
	return listenerId, meta
}

func (e *EventCenter[T]) Emit(eventName string, meta string, payload T) error {
	logger.Logger.Debug().Str("eventName", eventName).Str("meta", meta).Msg("emit")

	var runtimes []*listenerRuntime[T]
	shard := e.shardFor(eventName)
	shard.mu.Lock()
	listen := shard.events[eventName]
	if listen == nil {
		listen = &eventListeners[T]{listeners: make(map[string]*listenerRuntime[T])}
		shard.events[eventName] = listen
	}
	listen.currentMeta = meta
	if len(listen.listeners) > 0 {
		runtimes = make([]*listenerRuntime[T], 0, len(listen.listeners))
		for _, runtime := range listen.listeners {
			runtimes = append(runtimes, runtime)
		}
	}
	shard.mu.Unlock()

	for _, runtime := range runtimes {
		runtime.enqueue(payload)
	}
	return nil
}

func (e *EventCenter[T]) DeleteListener(eventName string, id string) {
	shard := e.shardFor(eventName)
	shard.mu.Lock()
	var runtime *listenerRuntime[T]
	if listen := shard.events[eventName]; listen != nil {
		runtime = listen.listeners[id]
		delete(listen.listeners, id)
	}
	shard.mu.Unlock()

	if runtime != nil {
		runtime.stop()
	}
}

func (e *EventCenter[T]) EventListenerCount(eventName string) int {
	shard := e.shardFor(eventName)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if listen := shard.events[eventName]; listen != nil {
		return len(listen.listeners)
	}
	return 0
}

func (e *EventCenter[T]) adjustQueueDepth(delta int64) {
	depth := e.queueDepth.Add(delta)
	if depth < 0 {
		depth = 0
		e.queueDepth.Store(0)
	}
	metric.SetEventBusListenerQueueDepth(float64(depth))
}
