package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeRollbackStore struct {
	task                 *sharedsubscription.ShareGroupTask
	completedTask        *sharedsubscription.ShareGroupTask
	processingTasks      []*sharedsubscription.ShareGroupTask
	queryProcessingErr   error
	rollbackErr          error
	rollbackN            int
	casSuccessN          int
	queryCompletedAlways bool
}

func (f *fakeRollbackStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	if f.task == nil {
		return nil, nil
	}
	return []*sharedsubscription.ShareGroupTask{f.task}, nil
}

func (f *fakeRollbackStore) QueryShareGroupTaskByMessageID(_ context.Context, _ string, _ uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	if len(statuses) == 1 && statuses[0] == sharedsubscription.TaskStatusCompleted {
		if f.queryCompletedAlways {
			return f.completedTask, nil
		}
		return nil, nil
	}
	return f.task, nil
}

func (f *fakeRollbackStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	if f.casSuccessN > 0 {
		f.casSuccessN--
		return true, nil
	}
	return false, nil
}

func (f *fakeRollbackStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	f.rollbackN++
	return nil
}

func (f *fakeRollbackStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	if f.queryProcessingErr != nil {
		return nil, f.queryProcessingErr
	}
	return f.processingTasks, nil
}

type fakeCursorStore struct{}

func (f *fakeCursorStore) ReadCursor(context.Context, string) (*store.DeliveryCursor, error) {
	return &store.DeliveryCursor{LastTS: time.Time{}, LastTaskID: uuid.Nil}, nil
}
func (f *fakeCursorStore) AdvanceCursor(context.Context, store.DeliveryCursor) error {
	return nil
}
func (f *fakeCursorStore) ReadTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*store.DeliveryTask, error) {
	return nil, nil
}
func (f *fakeCursorStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, nil
}

func TestRollbackServiceIdempotentRollback(t *testing.T) {
	task := &sharedsubscription.ShareGroupTask{
		TaskID:    uuid.New(),
		MessageID: uuid.New(),
		Status:    sharedsubscription.TaskStatusProcessing,
	}
	sharedStore := &fakeRollbackStore{task: task, casSuccessN: 1}
	service := NewRollbackService(sharedStore, &fakeCursorStore{}, nil)

	service.RollbackClientTasks(context.Background(), RollbackCommand{
		ClientID:   "c1",
		ShareGroup: "g1",
	})
	service.RollbackClientTasks(context.Background(), RollbackCommand{
		ClientID:   "c1",
		ShareGroup: "g1",
	})

	if sharedStore.rollbackN != 1 {
		t.Fatalf("expected rollback executed once, got %d", sharedStore.rollbackN)
	}
}

func TestRollbackClientTasksRecordsSemanticMetrics(t *testing.T) {
	task := &sharedsubscription.ShareGroupTask{
		TaskID:    uuid.New(),
		MessageID: uuid.New(),
		Status:    sharedsubscription.TaskStatusPending,
	}
	sharedStore := &fakeRollbackStore{task: task}
	service := NewRollbackService(sharedStore, &fakeCursorStore{}, nil)
	before := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("client_offline", "success"))

	service.RollbackClientTasks(context.Background(), RollbackCommand{
		ClientID:   "c1",
		ShareGroup: "g1",
	})

	if got := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("client_offline", "success")); got != before+1 {
		t.Fatalf("shared rollback success counter = %v, want %v", got, before+1)
	}
}

func TestRollbackClientTasksRecordsDuplicateGuardSkip(t *testing.T) {
	task := &sharedsubscription.ShareGroupTask{
		TaskID:    uuid.New(),
		MessageID: uuid.New(),
		Status:    sharedsubscription.TaskStatusProcessing,
	}
	sharedStore := &fakeRollbackStore{
		task:                 task,
		completedTask:        &sharedsubscription.ShareGroupTask{TaskID: uuid.New(), MessageID: task.MessageID, Status: sharedsubscription.TaskStatusCompleted},
		queryCompletedAlways: true,
	}
	service := NewRollbackService(sharedStore, &fakeCursorStore{}, nil)
	rollbackBefore := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("duplicate_guard", "skip"))
	duplicateBefore := testutil.ToFloat64(metric.DuplicateDeliveryTotal.WithLabelValues("shared", "runner"))

	service.RollbackClientTasks(context.Background(), RollbackCommand{
		ClientID:   "c1",
		ShareGroup: "g1",
	})

	if got := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("duplicate_guard", "skip")); got != rollbackBefore+1 {
		t.Fatalf("duplicate guard rollback counter = %v, want %v", got, rollbackBefore+1)
	}
	if got := testutil.ToFloat64(metric.DuplicateDeliveryTotal.WithLabelValues("shared", "runner")); got != duplicateBefore+1 {
		t.Fatalf("duplicate delivery counter = %v, want %v", got, duplicateBefore+1)
	}
}

func TestRequeueTimeoutTasksRecordsSemanticMetrics(t *testing.T) {
	task := &sharedsubscription.ShareGroupTask{
		TaskID:    uuid.New(),
		MessageID: uuid.New(),
		Status:    sharedsubscription.TaskStatusProcessing,
	}
	sharedStore := &fakeRollbackStore{processingTasks: []*sharedsubscription.ShareGroupTask{task}, casSuccessN: 1}
	service := NewRollbackService(sharedStore, &fakeCursorStore{}, nil)
	timeoutBefore := testutil.ToFloat64(metric.ProcessingTimeoutTotal.WithLabelValues("shared", "success"))
	rollbackBefore := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("processing_timeout", "success"))

	service.RequeueTimeoutTasks(context.Background(), "g1", time.Now())

	if got := testutil.ToFloat64(metric.ProcessingTimeoutTotal.WithLabelValues("shared", "success")); got != timeoutBefore+1 {
		t.Fatalf("processing timeout counter = %v, want %v", got, timeoutBefore+1)
	}
	if got := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("processing_timeout", "success")); got != rollbackBefore+1 {
		t.Fatalf("processing timeout rollback counter = %v, want %v", got, rollbackBefore+1)
	}
}

func TestRequeueTimeoutTasksRecordsQueryError(t *testing.T) {
	sharedStore := &fakeRollbackStore{queryProcessingErr: errors.New("query failed")}
	service := NewRollbackService(sharedStore, &fakeCursorStore{}, nil)
	timeoutBefore := testutil.ToFloat64(metric.ProcessingTimeoutTotal.WithLabelValues("shared", "error"))
	rollbackBefore := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("processing_timeout", "error"))

	service.RequeueTimeoutTasks(context.Background(), "g1", time.Now())

	if got := testutil.ToFloat64(metric.ProcessingTimeoutTotal.WithLabelValues("shared", "error")); got != timeoutBefore+1 {
		t.Fatalf("processing timeout error counter = %v, want %v", got, timeoutBefore+1)
	}
	if got := testutil.ToFloat64(metric.SharedRollbackTotal.WithLabelValues("processing_timeout", "error")); got != rollbackBefore+1 {
		t.Fatalf("processing timeout rollback error counter = %v, want %v", got, rollbackBefore+1)
	}
}
