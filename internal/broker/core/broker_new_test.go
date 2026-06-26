package core

import (
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/facade"
	"github.com/BAN1ce/skyTree/pkg/retry"
)

func TestNewBrokerReturnsErrorWhenServerConfigInvalid(t *testing.T) {
	_, err := NewBroker(
		config.Broker{
			Listen: []string{"mqtt://127.0.0.1:1883"},
		},
		config.Plugins{},
		config.Cluster{},
		config.DeliveryRunner{},
	)
	if err == nil {
		t.Fatal("expected error for invalid broker listener protocol")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewBrokerCreatesPublishRetryWorker(t *testing.T) {
	b, err := NewBroker(
		config.Broker{
			Listen: []string{"tcp://127.0.0.1:0"},
			MessageRetry: config.MessageRetry{
				MaxRetryCount:     3,
				Interval:          30 * time.Second,
				MaxTimeout:        300 * time.Second,
				SchedulerInterval: 250 * time.Millisecond,
			},
		},
		config.Plugins{},
		config.Cluster{},
		config.DeliveryRunner{},
	)
	if err != nil {
		t.Fatalf("NewBroker returned error: %v", err)
	}
	if b.publish.retry == nil {
		t.Fatal("expected broker publish retry schedule")
	}
	if b.publish.retryWorker == nil {
		t.Fatal("expected broker-owned publish retry worker")
	}
	if b.publish.retry != b.publish.retryWorker {
		t.Fatal("expected publish retry schedule to use broker-owned worker")
	}
	if got := b.publish.retryWorker.ScheduleInterval(); got != 250*time.Millisecond {
		t.Fatalf("publish retry scheduler interval = %s, want %s", got, 250*time.Millisecond)
	}
}

func TestNewBrokerHonorsInjectedPublishRetry(t *testing.T) {
	schedule := &newBrokerRetryScheduleStub{}
	b, err := NewBroker(
		config.Broker{
			Listen: []string{"tcp://127.0.0.1:0"},
		},
		config.Plugins{},
		config.Cluster{},
		config.DeliveryRunner{},
		WithPublishRetry(schedule),
	)
	if err != nil {
		t.Fatalf("NewBroker returned error: %v", err)
	}
	if b.publish.retry != schedule {
		t.Fatal("expected injected publish retry schedule")
	}
	if b.publish.retryWorker != nil {
		t.Fatal("expected no broker-owned worker when retry schedule is injected")
	}
}

type newBrokerRetryScheduleStub struct{}

func (newBrokerRetryScheduleStub) Create(*retry.Task) error { return nil }

func (newBrokerRetryScheduleStub) Delete(string) {}

var _ facade.RetrySchedule = (*newBrokerRetryScheduleStub)(nil)
