package facade

import (
	"github.com/BAN1ce/skyTree/pkg/retry"
)

type MockRetrySchedule struct {
}

func (m MockRetrySchedule) Create(task *retry.Task) error {
	return nil
}

func (m MockRetrySchedule) Delete(key string) {
}
