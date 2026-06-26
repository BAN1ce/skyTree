package retry

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/scheduler"
)

func NewSchedule(ctx context.Context, call, timeout func(task *Task) error, options ...Option) *DelayTaskSchedule {
	s := NewDelayTaskSchedule(ctx, call, timeout, options...)
	return s
}

type Option func(*DelayTaskSchedule)

func WithInterval(interval time.Duration) Option {
	return func(r *DelayTaskSchedule) {
		r.interval = interval
	}
}

func WithName(name string) Option {
	return func(schedule *DelayTaskSchedule) {
		schedule.scheduleName = name
	}
}

// WithDefaultRetryStrategy 设置默认重试策略
func WithDefaultRetryStrategy(strategy RetryStrategy) Option {
	return func(schedule *DelayTaskSchedule) {
		schedule.defaultRetryStrategy = strategy
	}
}

type DelayTaskSchedule struct {
	scheduleName         string
	interval             time.Duration
	scheduler            *scheduler.ActiveScheduler
	timeoutFunc          func(t *Task) error
	callFunc             func(t *Task) error
	defaultRetryStrategy RetryStrategy
}

func NewDelayTaskSchedule(ctx context.Context, callFunc, timeoutFunc func(t *Task) error, options ...Option) *DelayTaskSchedule {
	r := &DelayTaskSchedule{
		callFunc:             callFunc,
		timeoutFunc:          timeoutFunc,
		interval:             1 * time.Second,
		scheduleName:         "publish retry",
		defaultRetryStrategy: DefaultFixedStrategy,
	}
	for _, option := range options {
		option(r)
	}
	if r.interval == 0 {
		r.interval = time.Second
	}
	r.scheduler = scheduler.NewActiveScheduler(ctx, r.interval, func(_ string, task scheduler.Task) {
		if adapter, ok := task.(*TaskAdapter); ok {
			r.handleTask(adapter.GetTask())
		}
	})
	return r
}

func (d *DelayTaskSchedule) handleTask(t *Task) {
	if t == nil {
		return
	}

	// 检查是否超时
	if t.Data != nil && t.Data.RetryInfo != nil && t.Data.RetryInfo.IsTimeout() {
		if d.timeoutFunc != nil {
			if err := d.timeoutFunc(t); err != nil {
				if logger.Logger != nil {
					logger.Logger.Error().Err(err).Msg("timeoutFunc error")
				}
			}
		}
		return
	}

	// 执行任务
	if err := d.callFunc(t); err != nil {
		if logger.Logger != nil {
			logger.Logger.Error().Err(err).Msg("callFunc error")
		}

		// 检查是否需要重试
		if d.shouldRetry(t) {
			d.scheduleRetry(t)
		}
		return
	}

	// 任务成功，不需要重试
	if logger.Logger != nil {
		logger.Logger.Debug().
			Str("task_key", t.Key).
			Int("retry_count", t.RetryCount).
			Msg("task completed successfully")
	}
}

func (d *DelayTaskSchedule) shouldRetry(t *Task) bool {
	// 检查是否超过最大重试次数
	if t.IsRetryExceeded() {
		if logger.Logger != nil {
			logger.Logger.Warn().
				Str("task_key", t.Key).
				Int("retry_count", t.RetryCount).
				Int("max_retries", t.MaxRetries).
				Msg("task exceeded max retries")
		}
		return false
	}

	return true
}

func (d *DelayTaskSchedule) scheduleRetry(t *Task) {
	// 增加重试次数
	t.IncrementRetryCount()

	// 计算下次重试延迟时间
	nextDelay := t.CalculateNextDelay()

	if logger.Logger != nil {
		logger.Logger.Debug().
			Str("task_key", t.Key).
			Int("retry_count", t.RetryCount).
			Dur("next_delay", nextDelay).
			Msg("scheduling retry")
	}

	// 创建重试任务
	d.scheduler.Add(NewTaskAdapter(t, nextDelay))
}

func (d *DelayTaskSchedule) Start(ctx context.Context) error {
	d.scheduler.Start(ctx)
	return nil
}

func (d *DelayTaskSchedule) Create(task *Task) error {
	// 如果任务没有设置重试策略，使用默认策略
	if task.RetryStrategy == nil {
		task.RetryStrategy = d.defaultRetryStrategy
	}

	d.scheduler.Add(NewTaskAdapter(task, task.DelayTime))
	return nil
}

func (d *DelayTaskSchedule) Delete(key string) {
	d.scheduler.Remove(key)
}

// SetDefaultRetryStrategy 设置默认重试策略
func (d *DelayTaskSchedule) SetDefaultRetryStrategy(strategy RetryStrategy) {
	d.defaultRetryStrategy = strategy
}

func (d *DelayTaskSchedule) Interval() time.Duration {
	if d == nil {
		return 0
	}
	return d.interval
}
