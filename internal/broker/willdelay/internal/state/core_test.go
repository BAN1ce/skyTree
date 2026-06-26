package state

import (
	"sort"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

func TestCoreStoresWillDelayTasksByClientAndOwnerToken(t *testing.T) {
	core := NewCore(time.Second, 60)
	now := time.Now().UnixMicro()

	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "old-owner",
		ScheduledPublishTime: now + int64(time.Second/time.Microsecond),
	})
	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "new-owner",
		ScheduledPublishTime: now + int64(2*time.Second/time.Microsecond),
	})

	state := core.GetState()
	if len(state.GetTasks()) != 2 {
		t.Fatalf("tasks for the same ClientID but different owner tokens must not overwrite each other: %+v", state.GetTasks())
	}
}

func TestCoreDeleteTaskByOwnerLeavesOtherOwners(t *testing.T) {
	core := NewCore(time.Second, 60)
	now := time.Now().UnixMicro()

	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "old-owner",
		ScheduledPublishTime: now + int64(time.Second/time.Microsecond),
	})
	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "new-owner",
		ScheduledPublishTime: now + int64(2*time.Second/time.Microsecond),
	})

	core.DeleteTaskByOwner("client-a", "old-owner")
	state := core.GetState()
	if len(state.GetTasks()) != 1 {
		t.Fatalf("expected one remaining owner task, got %+v", state.GetTasks())
	}
	for _, task := range state.GetTasks() {
		if task.GetOwnerToken() != "new-owner" {
			t.Fatalf("expected new-owner task to remain, got %+v", task)
		}
	}
	dueTasks := core.GetDueTasks(now + int64(3*time.Second/time.Microsecond))
	if len(dueTasks) != 1 || dueTasks[0].GetOwnerToken() != "new-owner" {
		t.Fatalf("expected only new-owner task in scheduler, got %+v", dueTasks)
	}

	core.DeleteTask("client-a")
	if len(core.GetState().GetTasks()) != 0 {
		t.Fatalf("DeleteTask by ClientID must remove all owner-token variants")
	}
}

func TestCoreAddTaskUpdatesExistingOwnerSchedule(t *testing.T) {
	core := NewCore(time.Second, 60)
	now := time.Now().UnixMicro()

	if err := core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "owner-a",
		ScheduledPublishTime: now + int64(time.Hour/time.Microsecond),
	}); err != nil {
		t.Fatalf("add initial task: %v", err)
	}
	if err := core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "client-a",
		OwnerToken:           "owner-a",
		ScheduledPublishTime: now,
	}); err != nil {
		t.Fatalf("update task: %v", err)
	}

	dueTasks := core.GetDueTasks(now)
	if len(dueTasks) != 1 {
		t.Fatalf("expected updated task to be due once, got %+v", dueTasks)
	}
	if dueTasks[0].GetScheduledPublishTime() != now {
		t.Fatalf("expected updated scheduled time %d, got %d", now, dueTasks[0].GetScheduledPublishTime())
	}
	if len(core.GetState().GetTasks()) != 1 {
		t.Fatalf("updating existing owner task must keep one state entry, got %+v", core.GetState().GetTasks())
	}
}

func TestCoreGetDueTasksReturnsOnlyDueTasks(t *testing.T) {
	core := NewCore(time.Second, 60)
	now := time.Now().UnixMicro()

	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "past",
		OwnerToken:           "past-owner",
		ScheduledPublishTime: now - 1,
	})
	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "due",
		OwnerToken:           "due-owner",
		ScheduledPublishTime: now,
	})
	core.AddTask(&proto_will_delay.WillDelayTask{
		ClientID:             "future",
		OwnerToken:           "future-owner",
		ScheduledPublishTime: now + int64(time.Minute/time.Microsecond),
	})

	got := dueClientIDs(core.GetDueTasks(now))
	want := []string{"due", "past"}
	if !sameStrings(got, want) {
		t.Fatalf("expected due tasks %v, got %v", want, got)
	}
}

func TestCoreRecoverFromStateRebuildsDueTaskIndex(t *testing.T) {
	core := NewCore(time.Second, 60)
	now := time.Now().UnixMicro()
	state := &proto_will_delay.WillDelayState{
		Tasks: map[string]*proto_will_delay.WillDelayTask{
			"past": {
				ClientID:             "past",
				OwnerToken:           "past-owner",
				ScheduledPublishTime: now - 1,
			},
			"future": {
				ClientID:             "future",
				OwnerToken:           "future-owner",
				ScheduledPublishTime: now + int64(time.Minute/time.Microsecond),
			},
		},
	}

	core.RecoverFromState(state)

	got := dueClientIDs(core.GetDueTasks(now))
	want := []string{"past"}
	if !sameStrings(got, want) {
		t.Fatalf("expected recovered due tasks %v, got %v", want, got)
	}
}

func dueClientIDs(tasks []*proto_will_delay.WillDelayTask) []string {
	clientIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		clientIDs = append(clientIDs, task.GetClientID())
	}
	sort.Strings(clientIDs)
	return clientIDs
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
