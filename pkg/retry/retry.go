package retry

import (
	"context"
	"time"
)

func NewSchedule(ctx context.Context, options ...Option) *DelayTaskSchedule {
	return NewDelayTaskSchedule(ctx, func(key string, data interface{}) {
		if t, ok := data.(*Task); ok {
			if t.times <= t.MaxTimes && t.usedTime <= t.MaxTime {
				if t.Job == nil {
					return
				}
				t.Job()
				t.times++
				t.usedTime += t.IntervalTime
				t.IntervalTime *= 2
				return
			}
			if t.TimeoutJob != nil {
				t.TimeoutJob()
			}
		}
	}, options...)
}

type Task struct {
	MaxTimes     int
	MaxTime      time.Duration
	IntervalTime time.Duration
	times        int
	usedTime     time.Duration
	Key          string
	Job          func()
	TimeoutJob   func()
}

type Option func(*DelayTaskSchedule)

func WithInterval(interval time.Duration) Option {
	return func(r *DelayTaskSchedule) {
		r.interval = interval
	}
}
func WithSlotNum(slotNum int) Option {
	return func(r *DelayTaskSchedule) {
		r.slotNum = slotNum
	}
}

type DelayTaskSchedule struct {
	interval time.Duration
	slotNum  int
	tw       *TimeWheel
}

func NewDelayTaskSchedule(ctx context.Context, handler func(key string, data interface{}), options ...Option) *DelayTaskSchedule {
	r := &DelayTaskSchedule{}
	for _, option := range options {
		option(r)
	}
	if r.slotNum == 0 {
		r.slotNum = 1000
	}
	if r.interval == 0 {
		r.interval = time.Second
	}
	r.tw = NewTimeWheel(ctx, r.interval, r.slotNum, handler)
	return r
}
func (d *DelayTaskSchedule) Start() {
	go d.tw.Run()
}

func (d *DelayTaskSchedule) Create(task *Task) error {
	return d.tw.CreateTask(task.Key, task.IntervalTime, task)
}

func (d *DelayTaskSchedule) Delete(key string) {
	d.tw.DeleteTask(key)
}
