package willdelay_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/willdelay/memory"
	wdsm "github.com/BAN1ce/skyTree/internal/broker/willdelay/statemachine"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"google.golang.org/protobuf/proto"
)

func TestMemoryAddTaskRejectsMissingOwnerToken(t *testing.T) {
	center := memory.New()
	now := time.Now().UnixMicro()

	err := center.AddTask(context.Background(), &proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		ScheduledPublishTime: now,
	})
	if err == nil || !strings.Contains(err.Error(), "owner token") {
		t.Fatalf("expected owner token error, got %v", err)
	}

	dueTasks, err := center.GetDueTasks(context.Background(), now)
	if err != nil {
		t.Fatalf("get due tasks: %v", err)
	}
	if len(dueTasks) != 0 {
		t.Fatalf("task without owner token must not be scheduled, got %v", dueTasks)
	}
}

func TestStateMachineRejectsAddTaskMissingOwnerToken(t *testing.T) {
	sm := wdsm.New()
	req := &proto_will_delay.WillDelayRequest{
		Type: proto_will_delay.WillDelayRequestType_ADD_TASK,
		Task: &proto_will_delay.WillDelayTask{
			ClientID:             "client-a",
			ScheduledPublishTime: time.Now().UnixMicro(),
		},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	_, err = sm.Update(data)
	if err == nil || !strings.Contains(err.Error(), "owner token") {
		t.Fatalf("expected owner token error, got %v", err)
	}
}
