package retry

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"go.uber.org/zap"
	"time"
)

func NewSchedule(ctx context.Context, options ...Option) *DelayTaskSchedule {
	return NewDelayTaskSchedule(ctx, func(key string, data interface{}) {
		if t, ok := data.(*Task); ok {
			if t.times <= t.MaxTimes && (t.usedTime <= t.MaxTime || t.MaxTime == 0) {

				if t.Job == nil {
					return
				}
				t.Job(t)
				t.times++
				t.usedTime += t.IntervalTime
				t.IntervalTime *= 2
				return
			}
			if t.TimeoutJob != nil {
				t.TimeoutJob(t)
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
	Data         interface{}
	Job          func(task *Task)
	TimeoutJob   func(task *Task)
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

func WithName(name string) Option {
	return func(schedule *DelayTaskSchedule) {
		schedule.scheduleName = name
	}

}

type DelayTaskSchedule struct {
	scheduleName string
	interval     time.Duration
	slotNum      int
	tw           *TimeWheel
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
	logger.Logger.Debug("create task", zap.String("task", task.Key), zap.String("schedule", d.scheduleName))
	return d.tw.CreateTask(task.Key, task.IntervalTime, task)
}

func (d *DelayTaskSchedule) Delete(key string) {
	logger.Logger.Debug("delete task", zap.String("task", key), zap.String("schedule", d.scheduleName))
	d.tw.DeleteTask(key)
}
