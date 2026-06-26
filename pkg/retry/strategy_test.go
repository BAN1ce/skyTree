package retry

import (
	"testing"
	"time"
)

func TestFixedDelayStrategy(t *testing.T) {
	strategy := &FixedDelayStrategy{}
	originalDelay := 5 * time.Second
	taskKey := "test-task-1"

	for i := 0; i < 5; i++ {
		delay := strategy.CalculateDelay(i, originalDelay, taskKey)
		if delay != originalDelay {
			t.Errorf("FixedDelayStrategy should always return original delay, got %v, want %v", delay, originalDelay)
		}
	}
}

func TestExponentialBackoffStrategy(t *testing.T) {
	strategy := &ExponentialBackoffStrategy{
		MaxDelay: 60 * time.Second,
		Factor:   2.0,
	}
	originalDelay := 5 * time.Second
	taskKey := "test-task-2"

	// 测试指数增长
	expectedDelays := []time.Duration{
		5 * time.Second,  // 第0次重试
		10 * time.Second, // 第1次重试
		20 * time.Second, // 第2次重试
		40 * time.Second, // 第3次重试
		60 * time.Second, // 第4次重试（达到最大值）
	}

	for i, expected := range expectedDelays {
		delay := strategy.CalculateDelay(i, originalDelay, taskKey)
		if delay != expected {
			t.Errorf("ExponentialBackoffStrategy retry %d: got %v, want %v", i, delay, expected)
		}
	}
}

func TestLinearBackoffStrategy(t *testing.T) {
	strategy := &LinearBackoffStrategy{
		MaxDelay:  30 * time.Second,
		Increment: 5 * time.Second,
	}
	originalDelay := 5 * time.Second
	taskKey := "test-task-3"

	// 测试线性增长
	expectedDelays := []time.Duration{
		5 * time.Second,  // 第0次重试：5 + 0*5 = 5
		10 * time.Second, // 第1次重试：5 + 1*5 = 10
		15 * time.Second, // 第2次重试：5 + 2*5 = 15
		20 * time.Second, // 第3次重试：5 + 3*5 = 20
		25 * time.Second, // 第4次重试：5 + 4*5 = 25
		30 * time.Second, // 第5次重试：5 + 5*5 = 30（达到最大值）
	}

	for i, expected := range expectedDelays {
		delay := strategy.CalculateDelay(i, originalDelay, taskKey)
		if delay != expected {
			t.Errorf("LinearBackoffStrategy retry %d: got %v, want %v", i, delay, expected)
		}
	}
}

func TestJitterStrategy(t *testing.T) {
	baseStrategy := &FixedDelayStrategy{}
	strategy := &JitterStrategy{
		BaseStrategy: baseStrategy,
		JitterFactor: 0.1, // 10%抖动
	}
	originalDelay := 10 * time.Second
	taskKey := "test-task-4"

	// 测试抖动：相同taskKey应该产生相同的抖动
	delay1 := strategy.CalculateDelay(0, originalDelay, taskKey)
	delay2 := strategy.CalculateDelay(0, originalDelay, taskKey)

	if delay1 != delay2 {
		t.Errorf("JitterStrategy should produce consistent results for same task key, got %v and %v", delay1, delay2)
	}

	// 测试不同taskKey产生不同抖动
	delay3 := strategy.CalculateDelay(0, originalDelay, "different-task")

	// 虽然可能相同，但通常应该不同
	if delay1 == delay3 {
		t.Logf("Same delay for different keys (this is possible but unlikely): %v", delay1)
	}

	// 测试抖动范围（应该在原始延迟的±10%范围内）
	minDelay := time.Duration(float64(originalDelay) * 0.9)
	maxDelay := time.Duration(float64(originalDelay) * 1.1)

	if delay1 < minDelay || delay1 > maxDelay {
		t.Errorf("JitterStrategy delay out of range: got %v, expected between %v and %v", delay1, minDelay, maxDelay)
	}
}

func TestRetryStrategyFunc(t *testing.T) {
	// 测试函数类型适配器
	strategy := RetryStrategyFunc(func(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
		return originalDelay * time.Duration(retryCount+1)
	})

	originalDelay := 2 * time.Second
	taskKey := "test-task-5"

	expectedDelays := []time.Duration{
		2 * time.Second, // 第0次重试：2 * (0+1) = 2
		4 * time.Second, // 第1次重试：2 * (1+1) = 4
		6 * time.Second, // 第2次重试：2 * (2+1) = 6
	}

	for i, expected := range expectedDelays {
		delay := strategy.CalculateDelay(i, originalDelay, taskKey)
		if delay != expected {
			t.Errorf("RetryStrategyFunc retry %d: got %v, want %v", i, delay, expected)
		}
	}
}

func TestCreateExponentialBackoffWithJitter(t *testing.T) {
	strategy := CreateExponentialBackoffWithJitter(
		60*time.Second, // maxDelay
		2.0,            // factor
		0.2,            // jitterFactor
	)

	originalDelay := 5 * time.Second
	taskKey := "test-task-6"

	// 测试创建的策略是否正确类型
	if _, ok := strategy.(*JitterStrategy); !ok {
		t.Errorf("CreateExponentialBackoffWithJitter should return JitterStrategy, got %T", strategy)
	}

	// 测试第一次重试的延迟
	delay := strategy.CalculateDelay(0, originalDelay, taskKey)

	// 应该在原始延迟的±20%范围内
	minDelay := time.Duration(float64(originalDelay) * 0.8)
	maxDelay := time.Duration(float64(originalDelay) * 1.2)

	if delay < minDelay || delay > maxDelay {
		t.Errorf("First retry delay out of range: got %v, expected between %v and %v", delay, minDelay, maxDelay)
	}
}

func TestCreateCustomStrategy(t *testing.T) {
	strategy := CreateCustomStrategy(func(retryCount int, originalDelay time.Duration, taskKey string) time.Duration {
		// 自定义逻辑：前3次快速重试，后续慢速重试
		if retryCount < 3 {
			return originalDelay
		}
		return originalDelay * 10
	})

	originalDelay := 1 * time.Second
	taskKey := "test-task-7"

	testCases := []struct {
		retryCount int
		expected   time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 1 * time.Second},
		{3, 10 * time.Second},
		{4, 10 * time.Second},
	}

	for _, tc := range testCases {
		delay := strategy.CalculateDelay(tc.retryCount, originalDelay, taskKey)
		if delay != tc.expected {
			t.Errorf("CustomStrategy retry %d: got %v, want %v", tc.retryCount, delay, tc.expected)
		}
	}
}

func BenchmarkFixedDelayStrategy(b *testing.B) {
	strategy := &FixedDelayStrategy{}
	originalDelay := 5 * time.Second
	taskKey := "benchmark-task"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateDelay(i%10, originalDelay, taskKey)
	}
}

func BenchmarkExponentialBackoffStrategy(b *testing.B) {
	strategy := &ExponentialBackoffStrategy{
		MaxDelay: 60 * time.Second,
		Factor:   2.0,
	}
	originalDelay := 5 * time.Second
	taskKey := "benchmark-task"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateDelay(i%10, originalDelay, taskKey)
	}
}

func BenchmarkJitterStrategy(b *testing.B) {
	strategy := &JitterStrategy{
		BaseStrategy: &FixedDelayStrategy{},
		JitterFactor: 0.1,
	}
	originalDelay := 5 * time.Second
	taskKey := "benchmark-task"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateDelay(i%10, originalDelay, taskKey)
	}
}
