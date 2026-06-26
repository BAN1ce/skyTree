package manager

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type fakeCursorStore struct {
	cursor *store.DeliveryCursor
}

func (f *fakeCursorStore) ReadCursor(context.Context, string) (*store.DeliveryCursor, error) {
	return f.cursor, nil
}

func (f *fakeCursorStore) AdvanceCursor(context.Context, store.DeliveryCursor) error {
	panic("not used")
}

func (f *fakeCursorStore) ReadTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*store.DeliveryTask, error) {
	panic("not used")
}

func (f *fakeCursorStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	panic("not used")
}

type fakeSharedCursorStore struct {
	fakeCursorStore
	deleteCalls atomic.Int64
	clientID    string
	shareGroup  string
}

func (f *fakeSharedCursorStore) DeleteClientSharedTasks(_ context.Context, clientID string, shareGroup string) error {
	f.deleteCalls.Add(1)
	f.clientID = clientID
	f.shareGroup = shareGroup
	return nil
}

type fakeRollbackSharedStore struct {
	task          *sharedsubscription.ShareGroupTask
	completedTask *sharedsubscription.ShareGroupTask
	rollbackCalls atomic.Int64
	atomicCalls   atomic.Int64
	atomicUpdated bool
}

func (f *fakeRollbackSharedStore) EnsureSchema(context.Context) error { return nil }
func (f *fakeRollbackSharedStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	panic("not used")
}
func (f *fakeRollbackSharedStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeRollbackSharedStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	panic("not used")
}
func (f *fakeRollbackSharedStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	f.atomicCalls.Add(1)
	if f.atomicUpdated {
		return true, nil
	}
	return false, nil
}
func (f *fakeRollbackSharedStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	return []*sharedsubscription.ShareGroupTask{f.task}, nil
}
func (f *fakeRollbackSharedStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	f.rollbackCalls.Add(1)
	return nil
}
func (f *fakeRollbackSharedStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeRollbackSharedStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	panic("not used")
}
func (f *fakeRollbackSharedStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	panic("not used")
}
func (f *fakeRollbackSharedStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	panic("not used")
}
func (f *fakeRollbackSharedStore) QueryShareGroupTaskByMessageID(_ context.Context, _ string, _ uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	if len(statuses) == 1 && statuses[0] == sharedsubscription.TaskStatusCompleted {
		return f.completedTask, nil
	}
	return f.task, nil
}

type noopSharedStore struct{}

func (s *noopSharedStore) EnsureSchema(context.Context) error { return nil }
func (s *noopSharedStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	return nil
}
func (s *noopSharedStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *noopSharedStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	return nil
}
func (s *noopSharedStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	return false, nil
}
func (s *noopSharedStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *noopSharedStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	return nil
}
func (s *noopSharedStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *noopSharedStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}
func (s *noopSharedStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	return &sharedsubscription.ShareGroupCursor{}, nil
}
func (s *noopSharedStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	return nil
}
func (s *noopSharedStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

type recordingDeliveryEvent struct {
	sharedWakeCalls int
	sharedWake      delivery_notify.SharedWakePayload
}

func (e *recordingDeliveryEvent) AddListener(context.Context, string, delivery_notify.NotifyHandler) (string, string, error) {
	return "", "", nil
}

func (e *recordingDeliveryEvent) DeleteListener(context.Context, string, string) error {
	return nil
}

func (e *recordingDeliveryEvent) NotifyToNode(context.Context, uint64, string, []string, deliveryevent.Kind, []byte, map[string]delivery_notify.ClientDeliveryOptions) error {
	return nil
}

func (e *recordingDeliveryEvent) NotifySharedWake(_ context.Context, payload delivery_notify.SharedWakePayload) error {
	e.sharedWakeCalls++
	e.sharedWake = payload
	return nil
}

func TestManagerTracksClientShareGroups(t *testing.T) {
	m := NewSharedSubscriptionManager(nil, nil, nil, nil, nil, nil, nil, 0, nil, nil)
	m.rememberClientShareGroup("c1", "g1")
	m.rememberClientShareGroup("c1", "g2")
	m.rememberClientShareGroup("c1", "g1")

	groups, err := m.getClientShareGroups(t.Context(), "c1")
	if err != nil {
		t.Fatalf("getClientShareGroups: %v", err)
	}
	got := make(map[string]bool, len(groups))
	for _, group := range groups {
		got[group] = true
	}
	if len(got) != 2 || !got["g1"] || !got["g2"] {
		t.Fatalf("unexpected groups: %v", groups)
	}
}

func TestManagerUnsubscribeSharedFilterOnlyForgetsTargetGroup(t *testing.T) {
	m := NewSharedSubscriptionManager(nil, nil, nil, nil, nil, nil, nil, 0, nil, nil)
	m.rememberClientShareGroup("c1", "g1")
	m.rememberClientShareGroup("c1", "g2")

	if err := m.OnClientUnsubscribe(t.Context(), "c1", "g1", "a/b"); err != nil {
		t.Fatalf("OnClientUnsubscribe: %v", err)
	}

	groups, err := m.getClientShareGroups(t.Context(), "c1")
	if err != nil {
		t.Fatalf("getClientShareGroups: %v", err)
	}
	got := make(map[string]bool, len(groups))
	for _, group := range groups {
		got[group] = true
	}
	if len(got) != 1 || !got["g2"] {
		t.Fatalf("expected only g2 to remain, got %v", groups)
	}
}

func TestManagerUnsubscribeSharedFilterKeepsGroupWithRemainingFilter(t *testing.T) {
	m := NewSharedSubscriptionManager(nil, nil, nil, nil, nil, nil, nil, 0, nil, nil)
	m.rememberClientSharedTopicFilter("c1", "g1", "a/b")
	m.rememberClientSharedTopicFilter("c1", "g1", "a/c")

	if err := m.OnClientUnsubscribe(t.Context(), "c1", "g1", "a/b"); err != nil {
		t.Fatalf("OnClientUnsubscribe: %v", err)
	}

	groups, err := m.getClientShareGroups(t.Context(), "c1")
	if err != nil {
		t.Fatalf("getClientShareGroups: %v", err)
	}
	if len(groups) != 1 || groups[0] != "g1" {
		t.Fatalf("expected g1 to remain while another filter exists, got %v", groups)
	}
}

func TestNotifyTaskAppendedEnsuresConsumerAndBroadcastsWake(t *testing.T) {
	ev := &recordingDeliveryEvent{}
	m := NewSharedSubscriptionManager(
		&noopSharedStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		1,
		nil,
		nil,
		ev,
	)
	defer m.Stop()
	taskID := uuid.New()

	if err := m.NotifyTaskAppended(t.Context(), TaskAppendedEvent{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		TaskID:      taskID,
	}); err != nil {
		t.Fatalf("NotifyTaskAppended: %v", err)
	}

	m.consumersMux.RLock()
	consumer := m.consumers["g1"]
	m.consumersMux.RUnlock()
	if consumer == nil {
		t.Fatal("expected consumer to be created")
	}
	if ev.sharedWakeCalls != 1 {
		t.Fatalf("expected one shared wake broadcast, got %d", ev.sharedWakeCalls)
	}
	if ev.sharedWake.ShareGroup != "g1" || ev.sharedWake.TopicFilter != "a/b" || ev.sharedWake.TaskID != taskID.String() {
		t.Fatalf("unexpected shared wake payload: %+v", ev.sharedWake)
	}
}

func TestHandleRemoteTaskAppendedDoesNotBroadcastWake(t *testing.T) {
	ev := &recordingDeliveryEvent{}
	m := NewSharedSubscriptionManager(
		&noopSharedStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		1,
		nil,
		nil,
		ev,
	)
	defer m.Stop()

	if err := m.HandleRemoteTaskAppended(t.Context(), TaskAppendedEvent{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		TaskID:      uuid.New(),
	}); err != nil {
		t.Fatalf("HandleRemoteTaskAppended: %v", err)
	}

	m.consumersMux.RLock()
	consumer := m.consumers["g1"]
	m.consumersMux.RUnlock()
	if consumer == nil {
		t.Fatal("expected consumer to be created")
	}
	if ev.sharedWakeCalls != 0 {
		t.Fatalf("remote task should not rebroadcast, got %d broadcasts", ev.sharedWakeCalls)
	}
}

func TestTaskAppendedEventValidateRejectsEmptyFields(t *testing.T) {
	validID := uuid.New()
	tests := []struct {
		name  string
		event TaskAppendedEvent
	}{
		{name: "share group", event: TaskAppendedEvent{TopicFilter: "a/b", TaskID: validID}},
		{name: "topic filter", event: TaskAppendedEvent{ShareGroup: "g1", TaskID: validID}},
		{name: "task id", event: TaskAppendedEvent{ShareGroup: "g1", TopicFilter: "a/b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.event.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestManagerReadsClientDeliveryCursorForRollback(t *testing.T) {
	lastTaskID := uuid.New()
	lastTS := time.Unix(100, 200)
	m := NewSharedSubscriptionManager(
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeCursorStore{cursor: &store.DeliveryCursor{
			ClientID:   "c1",
			LastTS:     lastTS,
			LastTaskID: lastTaskID,
		}},
		nil,
		0,
		nil,
		nil,
	)

	cursor, err := m.getDeliveryCursor(t.Context(), "c1")
	if err != nil {
		t.Fatalf("getDeliveryCursor: %v", err)
	}
	if !cursor.LastTS.Equal(lastTS) || cursor.LastTaskID != lastTaskID {
		t.Fatalf("expected cursor (%v, %s), got (%v, %s)", lastTS, lastTaskID, cursor.LastTS, cursor.LastTaskID)
	}
}

func TestManagerRollbackDeletesStaleSharedClientTasks(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:     uuid.New(),
		ShareGroup: "g1",
		MessageID:  uuid.New(),
		Status:     sharedsubscription.TaskStatusPending,
	}
	sharedStore := &fakeRollbackSharedStore{task: task}
	cursorStore := &fakeSharedCursorStore{}
	m := NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		cursorStore,
		nil,
		0,
		nil,
		nil,
	)

	m.rollbackUnackedTasks(t.Context(), "c1", "g1")

	if got := sharedStore.rollbackCalls.Load(); got != 1 {
		t.Fatalf("expected one shared task rollback, got %d", got)
	}
	if got := cursorStore.deleteCalls.Load(); got != 1 {
		t.Fatalf("expected stale client shared delivery tasks to be deleted once, got %d", got)
	}
	if cursorStore.clientID != "c1" || cursorStore.shareGroup != "g1" {
		t.Fatalf("expected delete c1/g1, got %s/%s", cursorStore.clientID, cursorStore.shareGroup)
	}
}

func TestManagerStopCancelsPendingOfflineRollback(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:     uuid.New(),
		ShareGroup: "g1",
		MessageID:  uuid.New(),
		Status:     sharedsubscription.TaskStatusPending,
	}
	sharedStore := &fakeRollbackSharedStore{task: task}
	cursorStore := &fakeSharedCursorStore{}
	m := NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		cursorStore,
		nil,
		0,
		nil,
		nil,
	)
	m.rememberClientShareGroup("c1", "g1")

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	if err := m.OnClientOffline(context.Background(), "c1"); err != nil {
		t.Fatalf("OnClientOffline: %v", err)
	}
	cancel()
	m.Stop()

	time.Sleep(rollbackDelay + 100*time.Millisecond)

	if got := sharedStore.rollbackCalls.Load(); got != 0 {
		t.Fatalf("expected pending rollback to be canceled, got %d rollback calls", got)
	}
}

func TestManagerRollbackProcessingTaskCompletedRecheckNilDoesNotPanic(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:     uuid.New(),
		ShareGroup: "g1",
		MessageID:  uuid.New(),
		Status:     sharedsubscription.TaskStatusProcessing,
	}
	sharedStore := &fakeRollbackSharedStore{
		task:          task,
		completedTask: nil,
		atomicUpdated: true,
	}
	cursorStore := &fakeSharedCursorStore{}
	m := NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		cursorStore,
		nil,
		0,
		nil,
		nil,
	)

	m.rollbackUnackedTasks(t.Context(), "c1", "g1")

	if got := sharedStore.atomicCalls.Load(); got != 1 {
		t.Fatalf("expected one CAS attempt, got %d", got)
	}
	if got := sharedStore.rollbackCalls.Load(); got != 1 {
		t.Fatalf("expected rollback to continue after completed recheck miss, got %d", got)
	}
}

func TestManagerRollbackSkipsWhenProcessingCASFails(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:     uuid.New(),
		ShareGroup: "g1",
		MessageID:  uuid.New(),
		Status:     sharedsubscription.TaskStatusProcessing,
	}
	sharedStore := &fakeRollbackSharedStore{
		task:          task,
		completedTask: nil,
		atomicUpdated: false,
	}
	cursorStore := &fakeSharedCursorStore{}
	m := NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		cursorStore,
		nil,
		0,
		nil,
		nil,
	)

	m.rollbackUnackedTasks(t.Context(), "c1", "g1")

	if got := sharedStore.atomicCalls.Load(); got != 1 {
		t.Fatalf("expected one CAS attempt, got %d", got)
	}
	if got := sharedStore.rollbackCalls.Load(); got != 0 {
		t.Fatalf("expected rollback skipped when CAS fails, got %d", got)
	}
}
