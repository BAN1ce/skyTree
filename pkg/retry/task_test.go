package retry

import (
	"testing"
	"time"
)

func TestNewTask_Defaults(t *testing.T) {
	delay := 3 * time.Second
	task := NewTask("task-1", nil, "client-1", delay)

	if task.Key != "task-1" {
		t.Fatalf("unexpected key: %s", task.Key)
	}
	if task.DelayTime != delay {
		t.Fatalf("unexpected delay: %v", task.DelayTime)
	}
	if task.OriginalDelay != delay {
		t.Fatalf("unexpected original delay: %v", task.OriginalDelay)
	}
	if task.RetryCount != 0 {
		t.Fatalf("unexpected retry count: %d", task.RetryCount)
	}
	if task.RetryStrategy == nil {
		t.Fatal("default retry strategy should not be nil")
	}
}

func TestTask_RetryHelpers(t *testing.T) {
	task := NewTask("task-2", nil, "client-2", time.Second)
	task.SetMaxRetries(2)

	if task.IsRetryExceeded() {
		t.Fatal("retry should not be exceeded before increment")
	}

	task.IncrementRetryCount()
	if task.IsRetryExceeded() {
		t.Fatal("retry should not be exceeded at retryCount=1,max=2")
	}

	task.IncrementRetryCount()
	if !task.IsRetryExceeded() {
		t.Fatal("retry should be exceeded at retryCount=2,max=2")
	}
}
