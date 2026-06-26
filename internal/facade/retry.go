package facade

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/BAN1ce/skyTree/pkg/retry"
)

type RetrySchedule interface {
	Create(task *retry.Task) error
	Delete(key string)
}

/*

 */
// ----------------------------------  PublishRetry ----------------------------------//
/**

 */

type RetryWorker interface {
	CallRetry(task *retry.Task) error
	CallTimeout(task *retry.Task) error
}

type PublishRetry struct {
	schedule *retry.DelayTaskSchedule
	worker   RetryWorker
	option   []retry.Option
	cancel   context.CancelFunc
}

func NewPublishRetry(r RetryWorker, option ...retry.Option) *PublishRetry {
	if r == nil {
		return nil
	}
	return newPublishRetry(r.CallRetry, r.CallTimeout, option...)
}

func newPublishRetry(call, timeout func(t *retry.Task) error, option ...retry.Option) *PublishRetry {
	p := &PublishRetry{}
	p.schedule = retry.NewSchedule(context.Background(), call, timeout, option...)
	return p
}

// StartSchedule starts the underlying schedule under the caller-owned lifecycle context.
func (p *PublishRetry) StartSchedule(ctx context.Context) error {
	if p == nil || p.schedule == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, p.cancel = context.WithCancel(ctx)
	if err := p.schedule.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (p *PublishRetry) Create(task *retry.Task) error {
	if p == nil || p.schedule == nil {
		return nil
	}
	metric.PublishRetryTaskCurrent.Inc()
	metric.RecordPublishRetryAction(metric.PublishRetryActionCreate)
	return p.schedule.Create(task)
}

func (p *PublishRetry) Delete(key string) {
	if p == nil || p.schedule == nil {
		return
	}
	metric.RecordPublishRetryAction(metric.PublishRetryActionDelete)
	metric.PublishRetryTaskCurrent.Add(-1)
	p.schedule.Delete(key)
}

func (p *PublishRetry) Close() error {
	if p == nil || p.cancel == nil {
		return nil
	}
	p.cancel()
	p.cancel = nil
	return nil
}

func (p *PublishRetry) ScheduleInterval() time.Duration {
	if p == nil || p.schedule == nil {
		return 0
	}
	return p.schedule.Interval()
}
