package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/facade"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/google/uuid"
)

type recordingRetrySchedule struct {
	mu          sync.Mutex
	createCalls int
	lastTask    *retry.Task
}

func (r *recordingRetrySchedule) Create(task *retry.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createCalls++
	r.lastTask = task
	return nil
}

func (r *recordingRetrySchedule) Delete(string) {}

func (r *recordingRetrySchedule) created() (int, *retry.Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.createCalls, r.lastTask
}

type retryWriteFailConn struct {
	bufferConn
	err error
}

func (c *retryWriteFailConn) Write([]byte) (int, error) {
	return 0, c.err
}

func TestRetrySendDoesNotRequeueWhenWriteSucceeds(t *testing.T) {
	schedule := &recordingRetrySchedule{}
	conn := &bufferConn{}
	c := NewClient(conn, WithPublishRetry(schedule))
	c.ctx = context.Background()
	c.ID = "client-ok"

	msg := newRetrySendTestMessage()
	if err := c.RetrySend(msg); err != nil {
		t.Fatalf("RetrySend returned error: %v", err)
	}
	if calls, _ := schedule.created(); calls != 0 {
		t.Fatalf("retry schedule create calls = %d, want 0", calls)
	}
}

func TestRetrySendRequeuesWhenWriteFails(t *testing.T) {
	schedule := &recordingRetrySchedule{}
	writeErr := errors.New("write failed")
	c := NewClient(&retryWriteFailConn{err: writeErr}, WithPublishRetry(schedule))
	c.ctx = context.Background()
	c.ID = "client-fail"

	msg := newRetrySendTestMessage()
	err := c.RetrySend(msg)
	if err == nil {
		t.Fatal("expected RetrySend to return write error")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("unexpected RetrySend error: %v", err)
	}
	calls, task := schedule.created()
	if calls != 1 {
		t.Fatalf("retry schedule create calls = %d, want 1", calls)
	}
	if task == nil {
		t.Fatal("expected created retry task")
	}
	if task.Key != msg.RetryInfo.Key {
		t.Fatalf("retry task key = %q, want %q", task.Key, msg.RetryInfo.Key)
	}
}

func TestRetrySendWithoutPublishRetryReturnsWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	c := NewClient(&retryWriteFailConn{err: writeErr})
	c.ctx = context.Background()
	c.ID = "client-no-retry"

	err := c.RetrySend(newRetrySendTestMessage())
	if err == nil {
		t.Fatal("expected RetrySend to return write error")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("unexpected RetrySend error: %v", err)
	}
}

func TestRetrySendSkipsMalformedMessage(t *testing.T) {
	schedule := &recordingRetrySchedule{}
	c := NewClient(&bufferConn{}, WithPublishRetry(schedule))
	c.ctx = context.Background()
	c.ID = "client-malformed"

	tests := []struct {
		name    string
		message *brokerpublish.Message
	}{
		{
			name:    "nil message",
			message: nil,
		},
		{
			name: "nil publish",
			message: &brokerpublish.Message{
				SendClientID: "client-a",
			},
		},
		{
			name: "nil control packet",
			message: &brokerpublish.Message{
				SendClientID: "client-a",
				Publish: &packets.Publish{
					Topic: "retry/topic",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.RetrySend(tc.message); err != nil {
				t.Fatalf("RetrySend returned error: %v", err)
			}
		})
	}

	if calls, _ := schedule.created(); calls != 0 {
		t.Fatalf("retry schedule create calls = %d, want 0", calls)
	}
}

func newRetrySendTestMessage() *brokerpublish.Message {
	pub := &packets.Publish{
		PacketID: 1,
		Topic:    "retry/topic",
		QoS:      1,
		Payload:  []byte("payload"),
	}
	cp := packets.NewControlPacket(packets.PUBLISH)
	cp.Content = pub

	return &brokerpublish.Message{
		SendClientID:  "client-a",
		MessageID:     uuid.New(),
		ControlPacket: cp,
		Publish:       pub,
		RetryInfo: &brokerpublish.RetryInfo{
			Key:           "retry-key",
			IntervalTime:  20 * time.Millisecond,
			FirstPubTime:  time.Now(),
			MaxRetryCount: 3,
			Timeout:       time.Second,
		},
	}
}

var _ facade.RetrySchedule = (*recordingRetrySchedule)(nil)
