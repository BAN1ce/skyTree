package share

import (
	"container/list"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"sync"
)

type Queue struct {
	mux   sync.RWMutex
	queue list.List // *packet.Message
}

func NewQueue() *Queue {
	return &Queue{
		queue: list.List{},
	}
}

func (t *Queue) PutBackMessage(message []*packet.Message) {
	t.mux.Lock()
	defer t.mux.Unlock()
	for _, v := range message {
		t.queue.PushFront(v)
	}
}

func (t *Queue) AppendMessage(message []*packet.Message) {
	t.mux.Lock()
	defer t.mux.Unlock()
	for _, v := range message {
		t.queue.PushBack(v)
	}
}

func (t *Queue) PopQueue(n int) []*packet.Message {
	t.mux.Lock()
	defer t.mux.Unlock()
	var message []*packet.Message
	for i := 0; i < n; i++ {
		if t.queue.Len() == 0 {
			break
		}
		message = append(message, t.queue.Front().Value.(*packet.Message))
		t.queue.Remove(t.queue.Front())
	}
	return message
}

func (t *Queue) Len() int {
	t.mux.RLock()
	defer t.mux.RUnlock()
	return t.queue.Len()
}
