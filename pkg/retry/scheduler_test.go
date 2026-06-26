package retry

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
)

func BenchmarkCreateTask(b *testing.B) {
	schedule := NewDelayTaskSchedule(
		context.Background(),
		func(task *Task) error { return nil },
		func(task *Task) error { return nil },
		WithInterval(time.Second),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schedule.Create(NewTask(strconv.Itoa(i), &brokerpublish.Message{}, "client-benchmark", time.Second))
	}
}

func TestDelayTaskSchedule_Execute(t *testing.T) {
	const taskCount = 30

	var called atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	schedule := NewDelayTaskSchedule(
		ctx,
		func(task *Task) error {
			called.Add(1)
			return nil
		},
		func(task *Task) error { return nil },
		WithInterval(time.Second),
	)
	if err := schedule.Start(ctx); err != nil {
		t.Fatalf("start schedule: %v", err)
	}

	for i := 0; i < taskCount; i++ {
		task := NewTask(strconv.Itoa(i), &brokerpublish.Message{}, "client-test", time.Second)
		if err := schedule.Create(task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if called.Load() == taskCount {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("callback count mismatch: got=%d want=%d", called.Load(), taskCount)
}

func TestDelayTaskSchedule_Delete(t *testing.T) {
	var called atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	schedule := NewDelayTaskSchedule(
		ctx,
		func(task *Task) error {
			called.Add(1)
			return nil
		},
		func(task *Task) error { return nil },
		WithInterval(time.Second),
	)
	if err := schedule.Start(ctx); err != nil {
		t.Fatalf("start schedule: %v", err)
	}

	task := NewTask("task-delete", &brokerpublish.Message{}, "client-test", time.Second)
	if err := schedule.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	schedule.Delete(task.Key)

	time.Sleep(2 * time.Second)
	if called.Load() != 0 {
		t.Fatalf("deleted task should not be executed, got=%d", called.Load())
	}
}
