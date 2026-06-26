//go:build !race
// +build !race

package badgerstore

import (
	"context"
	"testing"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

func TestAdvanceDeliveryCursorDeletesAcknowledgedTasks(t *testing.T) {
	ctx := context.Background()
	s, err := NewLocalDeliveryQueueStore(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("NewLocalDeliveryQueueStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	msgID1 := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	msgID2 := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	taskID1 := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	taskID2 := uuid.MustParse("00000000-0000-0000-0000-000000000202")
	ts1 := time.Unix(10, 0)
	ts2 := time.Unix(11, 0)

	for _, task := range []brokerstore.DeliveryTask{
		{TS: ts1, TaskID: taskID1, ClientID: "client-a", MessageID: msgID1, DeliveryQoS: 1, RetainAsPublished: true},
		{TS: ts2, TaskID: taskID2, ClientID: "client-a", MessageID: msgID2, DeliveryQoS: 1, RetainAsPublished: true},
	} {
		if inserted, err := s.AppendDeliveryTask(ctx, task); err != nil {
			t.Fatalf("AppendDeliveryTask error: %v", err)
		} else if !inserted {
			t.Fatalf("AppendDeliveryTask inserted = false, want true")
		}
	}

	advanced, err := s.AdvanceDeliveryCursor(ctx, brokerstore.DeliveryCursor{
		UpdatedTS:  time.Now(),
		ClientID:   "client-a",
		LastTS:     ts1,
		LastTaskID: taskID1,
	})
	if err != nil {
		t.Fatalf("AdvanceDeliveryCursor error: %v", err)
	}
	if !advanced {
		t.Fatal("AdvanceDeliveryCursor advanced = false, want true")
	}

	tasks, err := s.ReadDeliveryTasks(ctx, "client-a", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadDeliveryTasks error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].TaskID != taskID2 {
		t.Fatalf("remaining task = %s, want %s", tasks[0].TaskID, taskID2)
	}
}

func TestResetClientDeliveryStateDeletesTasksAndCursor(t *testing.T) {
	ctx := context.Background()
	s, err := NewLocalDeliveryQueueStore(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("NewLocalDeliveryQueueStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	taskA := brokerstore.DeliveryTask{
		TS:                time.Unix(20, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000301"),
		ClientID:          "client-a",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000401"),
		DeliveryQoS:       1,
		RetainAsPublished: true,
	}
	taskB := brokerstore.DeliveryTask{
		TS:                time.Unix(21, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000302"),
		ClientID:          "client-b",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000402"),
		DeliveryQoS:       1,
		RetainAsPublished: true,
	}
	if inserted, err := s.AppendDeliveryTask(ctx, taskA); err != nil {
		t.Fatalf("AppendDeliveryTask client-a error: %v", err)
	} else if !inserted {
		t.Fatal("AppendDeliveryTask client-a inserted = false, want true")
	}
	if inserted, err := s.AppendDeliveryTask(ctx, taskB); err != nil {
		t.Fatalf("AppendDeliveryTask client-b error: %v", err)
	} else if !inserted {
		t.Fatal("AppendDeliveryTask client-b inserted = false, want true")
	}
	if advanced, err := s.AdvanceDeliveryCursor(ctx, brokerstore.DeliveryCursor{
		UpdatedTS:  time.Now(),
		ClientID:   "client-a",
		LastTS:     taskA.TS,
		LastTaskID: taskA.TaskID,
	}); err != nil {
		t.Fatalf("AdvanceDeliveryCursor error: %v", err)
	} else if !advanced {
		t.Fatal("AdvanceDeliveryCursor advanced = false, want true")
	}

	if err := s.ResetClientDeliveryState(ctx, "client-a"); err != nil {
		t.Fatalf("ResetClientDeliveryState error: %v", err)
	}

	tasksA, err := s.ReadDeliveryTasks(ctx, "client-a", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadDeliveryTasks client-a error: %v", err)
	}
	if len(tasksA) != 0 {
		t.Fatalf("expected client-a tasks deleted, got %d", len(tasksA))
	}
	cursorA, err := s.ReadDeliveryCursor(ctx, "client-a")
	if err != nil {
		t.Fatalf("ReadDeliveryCursor client-a error: %v", err)
	}
	if cursorA != nil {
		t.Fatalf("expected client-a cursor deleted, got %+v", cursorA)
	}
	tasksB, err := s.ReadDeliveryTasks(ctx, "client-b", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadDeliveryTasks client-b error: %v", err)
	}
	if len(tasksB) != 1 || tasksB[0].TaskID != taskB.TaskID {
		t.Fatalf("expected client-b task preserved, got %+v", tasksB)
	}
}

func TestLocalDeliveryQueueStoreSharedSubscriptionLifecycle(t *testing.T) {
	ctx := context.Background()
	s, err := NewLocalDeliveryQueueStore(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("NewLocalDeliveryQueueStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	var _ brokerstore.SharedSubscriptionStore = s

	task := &sharedsubscription.ShareGroupTask{
		TaskID:          uuid.MustParse("00000000-0000-0000-0000-000000000501"),
		ShareGroup:      "workers",
		TopicFilter:     "jobs/+",
		MessageID:       uuid.MustParse("00000000-0000-0000-0000-000000000601"),
		DeliveryQoS:     1,
		SubscriptionIDs: `[7]`,
		WinnerRAP:       true,
		Status:          sharedsubscription.TaskStatusPending,
	}
	ts := time.Unix(30, 0)
	if err := s.AppendShareGroupTask(ctx, ts, task); err != nil {
		t.Fatalf("AppendShareGroupTask error: %v", err)
	}

	tasks, err := s.ReadShareGroupTasks(ctx, "workers", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadShareGroupTasks error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != task.TaskID {
		t.Fatalf("expected pending task, got %+v", tasks)
	}

	updated, err := s.AtomicUpdateTaskStatus(
		ctx,
		task.TaskID,
		"workers",
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusProcessing,
	)
	if err != nil {
		t.Fatalf("AtomicUpdateTaskStatus pending->processing error: %v", err)
	}
	if !updated {
		t.Fatalf("expected pending->processing update")
	}

	processing, err := s.QueryProcessingTasksBefore(ctx, "workers", time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("QueryProcessingTasksBefore error: %v", err)
	}
	if len(processing) != 1 || processing[0].TaskID != task.TaskID {
		t.Fatalf("expected processing task, got %+v", processing)
	}

	if err := s.AppendShareGroupCursor(ctx, "workers", &sharedsubscription.ShareGroupCursor{
		ShareGroup:          "workers",
		LastProcessedTS:     ts,
		LastProcessedTaskID: task.TaskID,
		LeaderNodeID:        1,
		LastRenewal:         time.Unix(31, 0),
	}); err != nil {
		t.Fatalf("AppendShareGroupCursor error: %v", err)
	}
	cursor, err := s.ReadShareGroupCursor(ctx, "workers")
	if err != nil {
		t.Fatalf("ReadShareGroupCursor error: %v", err)
	}
	if cursor == nil || cursor.LastProcessedTaskID != task.TaskID {
		t.Fatalf("expected stored cursor, got %+v", cursor)
	}

	updated, err = s.AtomicUpdateTaskStatus(
		ctx,
		task.TaskID,
		"workers",
		sharedsubscription.TaskStatusProcessing,
		sharedsubscription.TaskStatusPending,
	)
	if err != nil {
		t.Fatalf("AtomicUpdateTaskStatus processing->pending error: %v", err)
	}
	if !updated {
		t.Fatalf("expected processing->pending update")
	}

	requeued, err := s.ReadShareGroupTasks(ctx, "workers", ts, task.TaskID, 10)
	if err != nil {
		t.Fatalf("ReadShareGroupTasks after requeue error: %v", err)
	}
	if len(requeued) != 1 || requeued[0].TaskID != task.TaskID {
		t.Fatalf("expected requeued task after cursor, got %+v", requeued)
	}

	if err := s.MarkShareGroupTaskProcessed(ctx, task.TaskID, "workers"); err != nil {
		t.Fatalf("MarkShareGroupTaskProcessed error: %v", err)
	}
	done, err := s.ReadShareGroupTasks(ctx, "workers", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadShareGroupTasks after processed error: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("expected completed task hidden from pending reads, got %+v", done)
	}
}

func TestLocalDeliveryQueueStoreBacklogSummary(t *testing.T) {
	ctx := context.Background()
	s, err := NewLocalDeliveryQueueStore(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("NewLocalDeliveryQueueStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	for _, task := range []brokerstore.DeliveryTask{
		{
			TS:        time.Unix(100, 0),
			TaskID:    uuid.MustParse("00000000-0000-0000-0000-000000010001"),
			ClientID:  "client-a",
			MessageID: uuid.MustParse("00000000-0000-0000-0000-000000020001"),
		},
		{
			TS:        time.Unix(101, 0),
			TaskID:    uuid.MustParse("00000000-0000-0000-0000-000000010002"),
			ClientID:  "client-a",
			MessageID: uuid.MustParse("00000000-0000-0000-0000-000000020002"),
		},
		{
			TS:        time.Unix(102, 0),
			TaskID:    uuid.MustParse("00000000-0000-0000-0000-000000010003"),
			ClientID:  "client-b",
			MessageID: uuid.MustParse("00000000-0000-0000-0000-000000020003"),
		},
	} {
		inserted, appendErr := s.AppendDeliveryTask(ctx, task)
		if appendErr != nil {
			t.Fatalf("AppendDeliveryTask error: %v", appendErr)
		}
		if !inserted {
			t.Fatalf("AppendDeliveryTask inserted=false for task=%s", task.TaskID)
		}
	}

	summary, err := s.DeliveryBacklogSummary(ctx)
	if err != nil {
		t.Fatalf("DeliveryBacklogSummary error: %v", err)
	}
	if summary.PendingTasks != 3 {
		t.Fatalf("PendingTasks=%d, want 3", summary.PendingTasks)
	}
	if summary.ActiveClients != 2 {
		t.Fatalf("ActiveClients=%d, want 2", summary.ActiveClients)
	}
}
