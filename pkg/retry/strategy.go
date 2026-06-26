package retry

import (
	"time"
)

// RetryStrategy 定义重试时间计算策略接口
type RetryStrategy interface {
	// CalculateDelay 计算下次重试的延迟时间
	// retryCount: 当前重试次数 (0表示第一次重试)
	// originalDelay: 原始配置的延迟时间
	// taskKey: 任务唯一标识
	CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration
}

// RetryStrategyFunc 函数类型适配器，允许普通函数实现RetryStrategy接口
type RetryStrategyFunc func(retryCount int, originalDelay time.Duration, taskKey string) time.Duration

func (f RetryStrategyFunc) CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
	return f(retryCount, originalDelay, taskKey)
}

// FixedDelayStrategy 固定延迟策略（当前的默认行为）
type FixedDelayStrategy struct{}

func (f *FixedDelayStrategy) CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
	return originalDelay
}

// ExponentialBackoffStrategy 指数退避策略
type ExponentialBackoffStrategy struct {
	MaxDelay time.Duration // 最大延迟时间
	Factor   float64       // 退避因子，默认2.0
}

func (e *ExponentialBackoffStrategy) CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
	factor := e.Factor
	if factor <= 0 {
		factor = 2.0
	}

	delay := originalDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * factor)
	}

	if e.MaxDelay > 0 && delay > e.MaxDelay {
		return e.MaxDelay
	}

	return delay
}

// LinearBackoffStrategy 线性退避策略
type LinearBackoffStrategy struct {
	MaxDelay  time.Duration // 最大延迟时间
	Increment time.Duration // 每次增加的延迟时间
}

func (l *LinearBackoffStrategy) CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
	increment := l.Increment
	if increment <= 0 {
		increment = originalDelay
	}

	delay := originalDelay + time.Duration(retryCount)*increment

	if l.MaxDelay > 0 && delay > l.MaxDelay {
		return l.MaxDelay
	}

	return delay
}

// JitterStrategy 抖动策略，在基础延迟上添加随机抖动
type JitterStrategy struct {
	BaseStrategy RetryStrategy
	JitterFactor float64 // 抖动因子 (0.0-1.0)，0.1表示±10%的抖动
}

func (j *JitterStrategy) CalculateDelay(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
	baseDelay := originalDelay
	if j.BaseStrategy != nil {
		baseDelay = j.BaseStrategy.CalculateDelay(retryCount, originalDelay, taskKey)
	}

	jitterFactor := j.JitterFactor
	if jitterFactor <= 0 {
		jitterFactor = 0.1
	}
	if jitterFactor > 1.0 {
		jitterFactor = 1.0
	}

	// 使用taskKey作为种子，确保相同任务的抖动一致
	seed := int64(0)
	for _, c := range taskKey {
		seed = seed*31 + int64(c)
	}

	// 简单的线性同余生成器
	seed = (seed*1103515245 + 12345) & 0x7fffffff
	jitter := float64(seed%1000) / 1000.0 // 0.0-1.0

	// 转换为 -jitterFactor 到 +jitterFactor 的范围
	jitter = (jitter - 0.5) * 2 * jitterFactor

	finalDelay := time.Duration(float64(baseDelay) * (1 + jitter))
	if finalDelay < 0 {
		finalDelay = baseDelay / 2
	}

	return finalDelay
}

// 预定义的常用策略
var (
	// DefaultFixedStrategy 默认固定延迟策略
	DefaultFixedStrategy = &FixedDelayStrategy{}

	// DefaultExponentialBackoffStrategy 默认指数退避策略
	DefaultExponentialBackoffStrategy = &ExponentialBackoffStrategy{
		MaxDelay: 5 * time.Minute,
		Factor:   2.0,
	}

	// DefaultLinearBackoffStrategy 默认线性退避策略
	DefaultLinearBackoffStrategy = &LinearBackoffStrategy{
		MaxDelay:  2 * time.Minute,
		Increment: 5 * time.Second,
	}
)

// CreateExponentialBackoffWithJitter 创建带抖动的指数退避策略
func CreateExponentialBackoffWithJitter(maxDelay time.Duration, factor float64, jitterFactor float64) RetryStrategy {
	return &JitterStrategy{
		BaseStrategy: &ExponentialBackoffStrategy{
			MaxDelay: maxDelay,
			Factor:   factor,
		},
		JitterFactor: jitterFactor,
	}
}

// CreateCustomStrategy 创建自定义策略
func CreateCustomStrategy(fn func(retryCount int, originalDelay time.Duration, taskKey string) time.Duration) RetryStrategy {
	return RetryStrategyFunc(fn)
}
