package wal

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

func TestLocalWALCenterRecoversPendingTasks(t *testing.T) {
	logger.LoadForTest()

	ctx := context.Background()
	baseDir := t.TempDir()
	scheduled := time.Now().Add(time.Minute).UnixMicro()

	c1, err := NewLocalWALCenter(baseDir, 0, 10000)
	if err != nil {
		t.Fatalf("new local wal center: %v", err)
	}
	if err := c1.AddTask(ctx, &proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		ScheduledPublishTime: scheduled,
		OwnerToken:           "owner-token",
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	if err := c1.Close(); err != nil {
		t.Fatalf("close first center: %v", err)
	}

	c2, err := NewLocalWALCenter(baseDir, 0, 10000)
	if err != nil {
		t.Fatalf("reopen local wal center: %v", err)
	}
	defer c2.Close()

	dueTasks, err := c2.GetDueTasks(ctx, scheduled+1)
	if err != nil {
		t.Fatalf("get due tasks: %v", err)
	}
	if len(dueTasks) != 1 || dueTasks[0].GetClientID() != "client-a" || dueTasks[0].GetOwnerToken() != "owner-token" {
		t.Fatalf("expected recovered due task for client-a, got %v", dueTasks)
	}
}
