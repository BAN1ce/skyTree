package facade

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/pkg/retry"
)

type retryWorkerStub struct{}

func (retryWorkerStub) CallRetry(*retry.Task) error   { return nil }
func (retryWorkerStub) CallTimeout(*retry.Task) error { return nil }

func TestNewPublishRetryRejectsNilWorker(t *testing.T) {
	if p := NewPublishRetry(nil); p != nil {
		t.Fatal("expected nil retry worker")
	}
}

func TestPublishRetryCloseBeforeStartDoesNotPanic(t *testing.T) {
	p := NewPublishRetry(retryWorkerStub{})
	if p == nil {
		t.Fatal("expected retry worker")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
}

func TestPublishRetryStartScheduleUsesReceiverSchedule(t *testing.T) {
	p1 := NewPublishRetry(retryWorkerStub{})
	if p1 == nil {
		t.Fatal("expected first retry worker")
	}
	_ = NewPublishRetry(retryWorkerStub{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p1.StartSchedule(ctx); err != nil {
		t.Fatalf("start schedule returned error: %v", err)
	}
	if err := p1.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
}

func TestPublishRetryStartScheduleReturnsAfterScheduleStarts(t *testing.T) {
	p := NewPublishRetry(retryWorkerStub{})
	if p == nil {
		t.Fatal("expected retry worker")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := p.StartSchedule(ctx); err != nil {
		t.Fatalf("start schedule returned error: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
}
