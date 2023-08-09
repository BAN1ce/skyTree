package config

import (
	"flag"
	"time"
)

var (
	nodeIDFlag = flag.Uint64("nodeID", 1, "node id")
	httpPort   = flag.Int("httpPort", 9526, "http port")
	brokerPort = flag.Int("brokerPort", 1883, "broker port")
)

type Config struct {
	Retry
	TimeWheel
	Tree
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
func (c Config) GetTree() Tree {
	return c.Tree

}
