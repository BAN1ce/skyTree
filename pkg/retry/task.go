package retry

import (
	"sync/atomic"
	"time"
)

type DelayTask struct {
	key    string
	delay  time.Duration
	data   interface{}
	circle atomic.Int64
}

func newDelayTask(key string, delay time.Duration, data interface{}) *DelayTask {
	return &DelayTask{
		data:  data,
		delay: delay,
		key:   key,
	}
}
