package combined

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

type closeFuncStore struct {
	brokerstore.DeliveryQueueStore
	brokerstore.MessagePayloadStore
	closeFn func() error
}

func (s closeFuncStore) Close() error {
	return s.closeFn()
}

type noopQueueStore struct{}

func (noopQueueStore) EnsureDeliverySchema(context.Context) error { return nil }
func (noopQueueStore) AppendDeliveryTask(context.Context, brokerstore.DeliveryTask) (bool, error) {
	return true, nil
}
func (noopQueueStore) DeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}
func (noopQueueStore) ReadDeliveryTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*brokerstore.DeliveryTask, error) {
	return nil, nil
}
func (noopQueueStore) AdvanceDeliveryCursor(context.Context, brokerstore.DeliveryCursor) (bool, error) {
	return true, nil
}
func (noopQueueStore) ReadDeliveryCursor(context.Context, string) (*brokerstore.DeliveryCursor, error) {
	return nil, nil
}
func (noopQueueStore) ResetClientDeliveryState(context.Context, string) error {
	return nil
}

type noopPayloadStore struct{}

func (noopPayloadStore) SaveMessagePayload(context.Context, brokerstore.MessagePayloadRecord) error {
	return nil
}
func (noopPayloadStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, brokerstore.ErrMessagePayloadNotFound
}

type countingQueueStore struct {
	noopQueueStore
	appendCalls atomic.Int64
}

func (s *countingQueueStore) AppendDeliveryTask(context.Context, brokerstore.DeliveryTask) (bool, error) {
	s.appendCalls.Add(1)
	return true, nil
}

type countingPayloadStore struct {
	saveCalls atomic.Int64
}

func (s *countingPayloadStore) SaveMessagePayload(context.Context, brokerstore.MessagePayloadRecord) error {
	s.saveCalls.Add(1)
	return nil
}

func (s *countingPayloadStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, brokerstore.ErrMessagePayloadNotFound
}

type recordingSharedQueueStore struct {
	noopQueueStore
	appended *sharedsubscription.ShareGroupTask
}

func (s *recordingSharedQueueStore) EnsureSchema(context.Context) error { return nil }
func (s *recordingSharedQueueStore) AppendShareGroupTask(_ context.Context, _ time.Time, task *sharedsubscription.ShareGroupTask) error {
	s.appended = task
	return nil
}
func (s *recordingSharedQueueStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *recordingSharedQueueStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	return nil
}
func (s *recordingSharedQueueStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	return false, nil
}
func (s *recordingSharedQueueStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *recordingSharedQueueStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	return nil
}
func (s *recordingSharedQueueStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *recordingSharedQueueStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}
func (s *recordingSharedQueueStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	return nil, nil
}
func (s *recordingSharedQueueStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	return nil
}
func (s *recordingSharedQueueStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

type summaryQueueStore struct {
	noopQueueStore
	summary brokerstore.DeliveryBacklogSummary
	err     error
}

func (s summaryQueueStore) DeliveryBacklogSummary(context.Context) (brokerstore.DeliveryBacklogSummary, error) {
	return s.summary, s.err
}

func TestClientDeliveryStoreCloseClosesBothStores(t *testing.T) {
	queueClosed := false
	payloadClosed := false

	s, err := NewClientDeliveryStore(
		closeFuncStore{
			DeliveryQueueStore: noopQueueStore{},
			closeFn: func() error {
				queueClosed = true
				return nil
			},
		},
		closeFuncStore{
			MessagePayloadStore: noopPayloadStore{},
			closeFn: func() error {
				payloadClosed = true
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("NewClientDeliveryStore error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !queueClosed || !payloadClosed {
		t.Fatalf("queueClosed=%v payloadClosed=%v, want both true", queueClosed, payloadClosed)
	}
}

func TestClientDeliveryStoreImplementsCombinedSubscriptionStore(t *testing.T) {
	var _ brokerstore.CombinedSubscriptionStore = (*ClientDeliveryStore)(nil)
}

func TestClientDeliveryStoreDelegatesSharedSubscriptionStore(t *testing.T) {
	ctx := context.Background()
	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "workers",
		TopicFilter: "jobs/+",
		MessageID:   uuid.New(),
		Status:      sharedsubscription.TaskStatusPending,
	}
	queue := &recordingSharedQueueStore{}
	s, err := NewClientDeliveryStore(queue, noopPayloadStore{})
	if err != nil {
		t.Fatalf("NewClientDeliveryStore error: %v", err)
	}

	if err := s.AppendShareGroupTask(ctx, time.Unix(10, 0), task); err != nil {
		t.Fatalf("AppendShareGroupTask error: %v", err)
	}
	if queue.appended == nil || queue.appended.TaskID != task.TaskID {
		t.Fatalf("expected shared task delegated to queue, got %+v", queue.appended)
	}
}

func TestClientDeliveryStoreConcurrentQueueAndPayloadDelegation(t *testing.T) {
	const (
		workers    = 12
		iterations = 80
	)
	queue := &countingQueueStore{}
	payload := &countingPayloadStore{}
	s, err := NewClientDeliveryStore(queue, payload)
	if err != nil {
		t.Fatalf("NewClientDeliveryStore error: %v", err)
	}

	ctx := context.Background()
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				task := brokerstore.DeliveryTask{
					TS:          time.Unix(int64(workerID*iterations+j), 0),
					TaskID:      uuid.New(),
					ClientID:    "client-a",
					MessageID:   uuid.New(),
					DeliveryQoS: 1,
				}
				if _, err := s.AppendDeliveryTask(ctx, task); err != nil {
					errCh <- err
					return
				}
				if err := s.SaveMessagePayload(ctx, brokerstore.MessagePayloadRecord{
					CreatedAt:         time.Now(),
					MessageID:         uuid.New(),
					PublishTopic:      "topic/a",
					PublisherClientID: "publisher",
					Payload:           []byte("payload"),
				}); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent delegate error: %v", err)
		}
	}

	want := int64(workers * iterations)
	if got := queue.appendCalls.Load(); got != want {
		t.Fatalf("queue append calls = %d, want %d", got, want)
	}
	if got := payload.saveCalls.Load(); got != want {
		t.Fatalf("payload save calls = %d, want %d", got, want)
	}
}

func TestClientDeliveryStoreDeliveryBacklogSummaryDelegatesToQueue(t *testing.T) {
	s, err := NewClientDeliveryStore(
		summaryQueueStore{
			summary: brokerstore.DeliveryBacklogSummary{
				PendingTasks:  9,
				ActiveClients: 4,
			},
		},
		noopPayloadStore{},
	)
	if err != nil {
		t.Fatalf("NewClientDeliveryStore error: %v", err)
	}

	summary, err := s.DeliveryBacklogSummary(context.Background())
	if err != nil {
		t.Fatalf("DeliveryBacklogSummary error: %v", err)
	}
	if summary.PendingTasks != 9 || summary.ActiveClients != 4 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestClientDeliveryStoreDeliveryBacklogSummaryReturnsUnsupported(t *testing.T) {
	s, err := NewClientDeliveryStore(noopQueueStore{}, noopPayloadStore{})
	if err != nil {
		t.Fatalf("NewClientDeliveryStore error: %v", err)
	}

	_, err = s.DeliveryBacklogSummary(context.Background())
	if err == nil {
		t.Fatal("expected unsupported summary error")
	}
	if !errors.Is(err, brokerstore.ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}
