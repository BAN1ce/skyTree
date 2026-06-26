package clusterhealth

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	events "github.com/kataras/go-events"
)

type testHealthEventEmitter struct {
	driver events.EventEmmiter
}

func (e testHealthEventEmitter) Emit(eventName string, payload interface{}) {
	if e.driver == nil {
		return
	}
	e.driver.Emit(events.EventName(eventName), payload)
}

func TestHealthCheckerFiltersToHostedClusters(t *testing.T) {
	hc := NewHealthChecker(context.Background(), &raft.Cluster{}, HealthCheckConfig{
		Enabled:    true,
		Interval:   time.Second,
		Timeout:    time.Second,
		MaxRetries: 1,
		HostedClusterIDs: []uint64{
			raft.ClusterIDSubCenter,
		},
	})

	if _, ok := hc.GetHealthStatus(raft.ClusterIDSubCenter); !ok {
		t.Fatalf("expected hosted system cluster status")
	}
	if _, ok := hc.GetHealthStatus(raft.ClusterIDKeyStore); ok {
		t.Fatalf("did not expect non-hosted system cluster status")
	}
	if _, ok := hc.GetHealthStatus(raft.ClusterIDSessionCenter); ok {
		t.Fatalf("did not expect non-hosted system cluster status")
	}
}

func TestHealthCheckerRecoveryTransitionEmitsOnlyRecoveryEvent(t *testing.T) {
	ensureTestLogger(t)

	driver := events.New()
	successCount := 0
	recoveryCount := 0
	driver.AddListener(HealthCheckSuccessEvent, func(...interface{}) {
		successCount++
	})
	driver.AddListener(HealthCheckRecoveryEvent, func(...interface{}) {
		recoveryCount++
	})

	hc := &HealthChecker{emitter: testHealthEventEmitter{driver: driver}}

	hc.emitStatusEvent(raft.ClusterIDKeyStore, HealthStatusUnknown, HealthStatusHealthy, nil)
	if successCount != 1 {
		t.Fatalf("success count = %d, want 1", successCount)
	}
	if recoveryCount != 0 {
		t.Fatalf("recovery count = %d, want 0", recoveryCount)
	}

	hc.emitStatusEvent(raft.ClusterIDKeyStore, HealthStatusUnhealthy, HealthStatusHealthy, nil)
	if successCount != 1 {
		t.Fatalf("success count = %d, want 1", successCount)
	}
	if recoveryCount != 0 {
		t.Fatalf("recovery count = %d, want 0", recoveryCount)
	}
}

func TestHealthCheckerEmitRecoveryEventUsesProvidedDowntime(t *testing.T) {
	ensureTestLogger(t)

	driver := events.New()
	eventCh := make(chan HealthRecoveryEvent, 1)
	driver.AddListener(HealthCheckRecoveryEvent, func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		switch payload := data[0].(type) {
		case HealthRecoveryEvent:
			eventCh <- payload
		case *HealthRecoveryEvent:
			if payload != nil {
				eventCh <- *payload
			}
		}
	})

	const downtime = 3 * time.Second
	hc := &HealthChecker{emitter: testHealthEventEmitter{driver: driver}}
	hc.emitRecoveryEvent(raft.ClusterIDSessionCenter, raft.ClusterNameSessionCenter, downtime)

	select {
	case payload := <-eventCh:
		if got := payload.ClusterID; got != raft.ClusterIDSessionCenter {
			t.Fatalf("cluster_id = %v, want %d", got, raft.ClusterIDSessionCenter)
		}
		if got := payload.ClusterName; got != raft.ClusterNameSessionCenter {
			t.Fatalf("cluster_name = %v, want %s", got, raft.ClusterNameSessionCenter)
		}
		if payload.Downtime != downtime {
			t.Fatalf("downtime = %v, want %v", payload.Downtime, downtime)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting recovery event")
	}
}

func ensureTestLogger(t *testing.T) {
	t.Helper()
	if logger.Logger == nil {
		logger.LoadForTest()
	}
}
