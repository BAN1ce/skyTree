package facade

import (
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"sync"
)

var (
	willDelay     RetrySchedule
	willDelayOnce sync.Once
)

func SingleWillDelay(option ...retry.Option) RetrySchedule {
	willDelayOnce.Do(func() {
		var s = retry.NewSchedule(config.GetRootContext(), option...)
		willDelay = s
	})
	return willDelay
}

func GetWillDelay() RetrySchedule {
	// FIXME: config
	var (
		cfg = config.GetTimeWheel()
	)
	return SingleWillDelay(retry.WithInterval(cfg.GetInterval()), retry.WithSlotNum(cfg.GetSlotNum()))
}
