package retry

import (
	"container/list"
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type callbackHandler func(key string, data interface{})

type op int

const (
	createOP = op(iota)
	deleteOP
)

type operation struct {
	op   op
	key  string
	task *DelayTask
}

func newDeleteOperation(key string) *operation {
	return &operation{
		op:  deleteOP,
		key: key,
	}
}
func newCreateOperation(task *DelayTask) *operation {
	return &operation{
		op:   createOP,
		task: task,
	}
}

type TimeWheel struct {
	interval      time.Duration
	ticker        *time.Ticker
	slots         []*list.List
	taskSlot      map[interface{}]int
	currentPos    atomic.Int64
	slotNum       int
	callback      callbackHandler
	operationChan chan *operation
	ctx           context.Context
}

func NewTimeWheel(ctx context.Context, interval time.Duration, slotNum int, handler callbackHandler) *TimeWheel {
	t := &TimeWheel{
		interval:      interval,
		slots:         make([]*list.List, slotNum),
		taskSlot:      make(map[interface{}]int),
		slotNum:       slotNum,
		callback:      handler,
		operationChan: make(chan *operation, 1000),
		ctx:           ctx,
	}
	for i := 0; i < t.slotNum; i++ {
		t.slots[i] = list.New()
	}
	return t
}
func (t *TimeWheel) Run() {
	go t.start()

}
func (t *TimeWheel) start() {
	t.ticker = time.NewTicker(t.interval)
	for {
		select {
		case <-t.ticker.C:
			t.doSlot()
		case op := <-t.operationChan:
			if op.op == createOP {
				t.addTask(op.task)
			} else if op.op == deleteOP {
				t.removeTask(op.key)
			}
		case <-t.ctx.Done():
			t.ticker.Stop()
			return
		}
	}
}

func (t *TimeWheel) doSlot() {
	l := t.slots[t.currentPos.Load()]
	t.scanAndRunTask(l)
	if t.currentPos.Load() == int64(t.slotNum-1) {
		t.currentPos.Store(0)
	} else {
		t.currentPos.Add(1)
	}
}

func (t *TimeWheel) scanAndRunTask(l *list.List) {
	for e := l.Front(); e != nil; e = e.Next() {
		task := e.Value.(*DelayTask)
		if task.circle.Load() > 0 {
			task.circle.Add(-1)
			continue
		}
		go t.callback(task.key, task.data)
		l.Remove(e)
		if task.key != "" {
			delete(t.taskSlot, task.key)
		}
	}
}

func (t *TimeWheel) CreateTask(key string, delay time.Duration, data interface{}) error {
	select {
	case <-t.ctx.Done():
		return errors.New("closed")
	case t.operationChan <- newCreateOperation(newDelayTask(key, delay, data)):
	}
	return nil
}

func (t *TimeWheel) DeleteTask(key string) {
	select {
	case <-t.ctx.Done():
		return
	case t.operationChan <- newDeleteOperation(key):
	}
}

func (t *TimeWheel) removeTask(key interface{}) {
	position, ok := t.taskSlot[key]
	if !ok {
		return
	}
	l := t.slots[position]
	for e := l.Front(); e != nil; e = e.Next() {
		task := e.Value.(*DelayTask)
		if task.key == key {
			delete(t.taskSlot, task.key)
			l.Remove(e)
			return
		}
	}
}

func (t *TimeWheel) addTask(task *DelayTask) {
	pos, circle := t.getPositionAndCircle(task.delay)
	task.circle.Store(int64(circle))
	t.slots[pos].PushBack(task)
	if task.key != "" {
		t.taskSlot[task.key] = pos
	}
}

func (t *TimeWheel) getPositionAndCircle(d time.Duration) (pos int, circle int) {
	var (
		delaySeconds    = int(d.Seconds())
		intervalSeconds = int(t.interval.Seconds())
		currentPos      = int(t.currentPos.Load())
	)
	circle = delaySeconds / intervalSeconds / t.slotNum
	pos = (currentPos + delaySeconds/intervalSeconds) % t.slotNum
	return
}
