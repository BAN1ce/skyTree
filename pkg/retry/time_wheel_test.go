package retry

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkCreateTask(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := NewTimeWheel(ctx, time.Second, 1000, func(key string, data interface{}) {
		time.Sleep(10 * time.Millisecond)
	})
	tm.Run()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tm.CreateTask(strconv.Itoa(i), time.Duration(i)*time.Second, nil); err != nil {
			b.Errorf("create task error: %v", err)
		}
	}
}
