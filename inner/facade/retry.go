package facade

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"sync"
)

type RetrySchedule interface {
	Create(task *retry.Task) error
	Delete(key string)
}

var (
	PublishRetry RetrySchedule
	PubRelRetry  RetrySchedule
)
var (
	OncePublishRetry sync.Once
	OncePubRelRetry  sync.Once
)

func SinglePublishRetry(option ...retry.Option) RetrySchedule {
	OncePublishRetry.Do(func() {
		var s = retry.NewSchedule(config.GetRootContext(), option...)
		s.Start()
		PublishRetry = s

	})
	return PublishRetry
}

func GetPublishRetry() RetrySchedule {
	var (
		cfg = config.GetTimeWheel()
	)
	return SinglePublishRetry(retry.WithInterval(cfg.GetInterval()), retry.WithSlotNum(cfg.GetSlotNum()))
}

func SinglePubRelRetry(ctx context.Context, option ...retry.Option) RetrySchedule {
	OncePubRelRetry.Do(func() {
		var s = retry.NewSchedule(ctx, option...)
		PubRelRetry = s
	})
	return PubRelRetry
}

func GetPubRelRetry() RetrySchedule {
	return SinglePubRelRetry(config.GetRootContext())
}
