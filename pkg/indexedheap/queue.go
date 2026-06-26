package indexedheap

import (
	"container/heap"
	"sort"
)

type entry[K comparable, V any] struct {
	key      K
	priority int64
	value    V
	index    int
	seq      uint64
}

type priorityHeap[K comparable, V any] []*entry[K, V]

func (h priorityHeap[K, V]) Len() int {
	return len(h)
}

func (h priorityHeap[K, V]) Less(i, j int) bool {
	if h[i].priority == h[j].priority {
		return h[i].seq < h[j].seq
	}
	return h[i].priority < h[j].priority
}

func (h priorityHeap[K, V]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityHeap[K, V]) Push(x any) {
	item := x.(*entry[K, V])
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *priorityHeap[K, V]) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}

// Queue is an indexed min-heap keyed by K and ordered by int64 priority.
type Queue[K comparable, V any] struct {
	items priorityHeap[K, V]
	byKey map[K]*entry[K, V]
	next  uint64
}

func New[K comparable, V any]() *Queue[K, V] {
	q := &Queue[K, V]{
		items: priorityHeap[K, V]{},
		byKey: make(map[K]*entry[K, V]),
	}
	heap.Init(&q.items)
	return q
}

func (q *Queue[K, V]) Len() int {
	if q == nil {
		return 0
	}
	return q.items.Len()
}

func (q *Queue[K, V]) Upsert(key K, priority int64, value V) {
	if item, ok := q.byKey[key]; ok {
		item.priority = priority
		item.value = value
		heap.Fix(&q.items, item.index)
		return
	}

	item := &entry[K, V]{
		key:      key,
		priority: priority,
		value:    value,
		seq:      q.next,
	}
	q.next++
	heap.Push(&q.items, item)
	q.byKey[key] = item
}

func (q *Queue[K, V]) Remove(key K) bool {
	if q == nil {
		return false
	}
	item, ok := q.byKey[key]
	if !ok {
		return false
	}
	heap.Remove(&q.items, item.index)
	delete(q.byKey, key)
	return true
}

func (q *Queue[K, V]) Get(key K) (V, bool) {
	var zero V
	if q == nil {
		return zero, false
	}
	item, ok := q.byKey[key]
	if !ok {
		return zero, false
	}
	return item.value, true
}

func (q *Queue[K, V]) Peek() (K, int64, V, bool) {
	var zeroKey K
	var zeroValue V
	if q == nil || q.items.Len() == 0 {
		return zeroKey, 0, zeroValue, false
	}
	item := q.items[0]
	return item.key, item.priority, item.value, true
}

func (q *Queue[K, V]) ValuesAtOrBefore(priority int64) []V {
	if q == nil || q.items.Len() == 0 {
		return []V{}
	}

	due := make([]*entry[K, V], 0)
	q.collectAtOrBefore(0, priority, &due)
	sort.Slice(due, func(i, j int) bool {
		if due[i].priority == due[j].priority {
			return due[i].seq < due[j].seq
		}
		return due[i].priority < due[j].priority
	})

	out := make([]V, 0, len(due))
	for _, item := range due {
		out = append(out, item.value)
	}
	return out
}

func (q *Queue[K, V]) collectAtOrBefore(index int, priority int64, out *[]*entry[K, V]) {
	if index >= q.items.Len() {
		return
	}
	item := q.items[index]
	if item.priority > priority {
		return
	}
	*out = append(*out, item)
	q.collectAtOrBefore(2*index+1, priority, out)
	q.collectAtOrBefore(2*index+2, priority, out)
}
