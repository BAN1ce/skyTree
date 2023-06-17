package config

import "time"

type Config struct {
	Retry
	TimeWheel
}

func GetPubMaxQos() uint8 {
	return 1
}

type Retry struct {
	MaxRetryCount int
	MaxTime       time.Duration
	Interval      time.Duration
}

func (r Retry) GetMaxRetryCount() int {
	return r.MaxRetryCount
}

func (r Retry) GetMaxTime() time.Duration {
	return r.MaxTime
}

func (r Retry) GetInterval() time.Duration {
	return r.Interval
}

func GetRetry() Retry {
	return Retry{
		MaxRetryCount: 3,
		Interval:      3 * time.Second,
	}
}
