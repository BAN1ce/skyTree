package retry

import (
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
)

type Task struct {
	Key           string
	Data          *brokerpublish.Message
	ClientID      string
	DelayTime     time.Duration
	RetryStrategy RetryStrategy // 重试策略
	RetryCount    int           // 当前重试次数
	MaxRetries    int           // 最大重试次数，0表示无限重试
	OriginalDelay time.Duration // 原始延迟时间
}

func NewTask(Key string, Data *brokerpublish.Message, ClientID string, DelayTime time.Duration) *Task {
	return &Task{
		Key:           Key,
		Data:          Data,
		ClientID:      ClientID,
		DelayTime:     DelayTime,
		RetryStrategy: DefaultFixedStrategy, // 默认使用固定延迟策略
		RetryCount:    0,
		MaxRetries:    0,
		OriginalDelay: DelayTime,
	}
}

// NewTaskWithStrategy 创建带重试策略的任务
func NewTaskWithStrategy(Key string, Data *brokerpublish.Message, ClientID string, DelayTime time.Duration, strategy RetryStrategy, maxRetries int) *Task {
	return &Task{
		Key:           Key,
		Data:          Data,
		ClientID:      ClientID,
		DelayTime:     DelayTime,
		RetryStrategy: strategy,
		RetryCount:    0,
		MaxRetries:    maxRetries,
		OriginalDelay: DelayTime,
	}
}

// IsRetryExceeded 检查是否超过最大重试次数
func (t *Task) IsRetryExceeded() bool {
	if t.MaxRetries == 0 {
		return false // 0表示无限重试
	}
	return t.RetryCount >= t.MaxRetries
}

// IncrementRetryCount 增加重试次数
func (t *Task) IncrementRetryCount() {
	t.RetryCount++
}

// CalculateNextDelay 计算下次重试的延迟时间
func (t *Task) CalculateNextDelay() time.Duration {
	if t.RetryStrategy == nil {
		return t.DelayTime
	}
	return t.RetryStrategy.CalculateDelay(t.RetryCount, t.OriginalDelay, t.Key)
}

// SetRetryStrategy 设置重试策略
func (t *Task) SetRetryStrategy(strategy RetryStrategy) {
	t.RetryStrategy = strategy
}

// SetMaxRetries 设置最大重试次数
func (t *Task) SetMaxRetries(maxRetries int) {
	t.MaxRetries = maxRetries
}
