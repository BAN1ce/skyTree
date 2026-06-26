package scheduler

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// Task describes a schedulable item keyed by a unique identifier.
type Task interface {
	GetKey() string       // Unique task identifier.
	GetExpireTime() int64 // Expiration time as a Unix microsecond timestamp.
}

// Scheduler exposes the scheduling operations used by retry and will-delay code.
type Scheduler interface {
	Add(task Task)
	Remove(key string) bool
	GetExpired(currentTime int64) []Task
	Advance()
	Size() int
	CurrentPosition() int64
}

// PassiveScheduler is an indexed min-heap scheduler.
type PassiveScheduler struct {
	mux        sync.RWMutex
	items      taskHeap
	entries    map[string]*heapItem
	interval   time.Duration
	currentPos int64
}

type heapItem struct {
	task  Task
	index int
}

type taskHeap []*heapItem

func (h taskHeap) Len() int {
	return len(h)
}

func (h taskHeap) Less(i, j int) bool {
	leftExpire := h[i].task.GetExpireTime()
	rightExpire := h[j].task.GetExpireTime()
	if leftExpire == rightExpire {
		return h[i].task.GetKey() < h[j].task.GetKey()
	}
	return leftExpire < rightExpire
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x any) {
	item := x.(*heapItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1
	item.task = nil
	*h = old[:n-1]
	return item
}

// NewPassiveScheduler creates a passive scheduler.
func NewPassiveScheduler(interval time.Duration) *PassiveScheduler {
	if interval <= 0 {
		interval = time.Second
	}

	tw := &PassiveScheduler{
		items:      taskHeap{},
		entries:    make(map[string]*heapItem),
		interval:   interval,
		currentPos: 0,
	}

	return tw
}

// Add adds or replaces a task by key.
func (tw *PassiveScheduler) Add(task Task) {
	if task == nil {
		return
	}

	key := task.GetKey()
	if key == "" {
		return
	}

	tw.mux.Lock()
	defer tw.mux.Unlock()

	if item, exists := tw.entries[key]; exists {
		item.task = task
		heap.Fix(&tw.items, item.index)
		return
	}

	item := &heapItem{task: task}
	heap.Push(&tw.items, item)
	tw.entries[key] = item
}

// Remove deletes a task by key.
func (tw *PassiveScheduler) Remove(key string) bool {
	if key == "" {
		return false
	}

	tw.mux.Lock()
	defer tw.mux.Unlock()

	item, ok := tw.entries[key]
	if !ok {
		return false
	}

	heap.Remove(&tw.items, item.index)
	delete(tw.entries, key)
	return true
}

// GetExpired returns expired tasks without removing them.
func (tw *PassiveScheduler) GetExpired(currentTime int64) []Task {
	tw.mux.RLock()
	defer tw.mux.RUnlock()

	if len(tw.items) == 0 || tw.items[0].task.GetExpireTime() > currentTime {
		return nil
	}

	expiredTasks := []Task{}
	stack := []int{0}
	for len(stack) > 0 {
		last := len(stack) - 1
		index := stack[last]
		stack = stack[:last]

		if index >= len(tw.items) {
			continue
		}
		task := tw.items[index].task
		if task.GetExpireTime() > currentTime {
			continue
		}

		expiredTasks = append(expiredTasks, task)
		stack = append(stack, 2*index+1, 2*index+2)
	}

	return expiredTasks
}

// Advance records one scheduler tick for compatibility with active scheduling.
func (tw *PassiveScheduler) Advance() {
	tw.mux.Lock()
	defer tw.mux.Unlock()

	tw.currentPos++
}

// Size returns the number of scheduled tasks.
func (tw *PassiveScheduler) Size() int {
	tw.mux.RLock()
	defer tw.mux.RUnlock()

	return len(tw.entries)
}

// CurrentPosition returns the number of manual or active scheduler ticks.
func (tw *PassiveScheduler) CurrentPosition() int64 {
	tw.mux.RLock()
	defer tw.mux.RUnlock()

	return tw.currentPos
}

// ActiveScheduler runs the passive scheduler on a ticker.
type ActiveScheduler struct {
	*PassiveScheduler
	ticker   *time.Ticker
	callback func(key string, task Task)
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	startMux sync.Mutex
}

// NewActiveScheduler creates an active scheduler for retry-style callbacks.
func NewActiveScheduler(ctx context.Context, interval time.Duration, callback func(key string, task Task)) *ActiveScheduler {
	passive := NewPassiveScheduler(interval)
	ctx, cancel := context.WithCancel(ctx)

	return &ActiveScheduler{
		PassiveScheduler: passive,
		callback:         callback,
		ctx:              ctx,
		cancel:           cancel,
		started:          false,
	}
}

// Start begins the active scheduler loop.
func (tw *ActiveScheduler) Start(ctx context.Context) {
	tw.startMux.Lock()
	defer tw.startMux.Unlock()

	if tw.started {
		return
	}

	tw.started = true
	tw.ticker = time.NewTicker(tw.interval)

	go tw.run(ctx)
}

// Stop terminates the active scheduler loop.
func (tw *ActiveScheduler) Stop() {
	tw.startMux.Lock()
	defer tw.startMux.Unlock()

	if !tw.started {
		return
	}

	tw.started = false
	if tw.ticker != nil {
		tw.ticker.Stop()
	}
	tw.cancel()
}

func (tw *ActiveScheduler) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tw.ctx.Done():
			return
		case <-tw.ticker.C:
			tw.doSlot()
		}
	}
}

func (tw *ActiveScheduler) doSlot() {
	currentTime := time.Now().UnixMicro()
	expiredTasks := tw.GetExpired(currentTime)

	for _, task := range expiredTasks {
		if tw.callback != nil {
			tw.callback(task.GetKey(), task)
		}
		tw.Remove(task.GetKey())
	}

	tw.Advance()
}
