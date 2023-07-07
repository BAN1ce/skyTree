package retry

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkCreateTask(b *testing.B) {
	tm := NewTimeWheel(context.TODO(), time.Second, 1000, func(key string, data interface{}) {
		time.Sleep(10 * time.Millisecond)
	})
	tm.Run()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.CreateTask(strconv.Itoa(i), time.Duration(i)*time.Second, nil)
	}
}
