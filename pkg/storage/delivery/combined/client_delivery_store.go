package combined

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	broker_store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

// ClientDeliveryStore composes a DeliveryQueueStore and a MessagePayloadStore into the unified interface.
type ClientDeliveryStore struct {
	queue   broker_store.DeliveryQueueStore
	payload broker_store.MessagePayloadStore
	shared  broker_store.SharedSubscriptionStore
}

var _ broker_store.ClientDeliveryStore = (*ClientDeliveryStore)(nil)
var _ broker_store.CombinedSubscriptionStore = (*ClientDeliveryStore)(nil)
var _ broker_store.DeliveryBacklogSummaryStore = (*ClientDeliveryStore)(nil)

func NewClientDeliveryStore(queue broker_store.DeliveryQueueStore, payload broker_store.MessagePayloadStore) (*ClientDeliveryStore, error) {
	if queue == nil {
		return nil, fmt.Errorf("queue store is nil")
	}
	if payload == nil {
		return nil, fmt.Errorf("payload store is nil")
	}
	shared, _ := queue.(broker_store.SharedSubscriptionStore)
	return &ClientDeliveryStore{
		queue:   queue,
		payload: payload,
		shared:  shared,
	}, nil
}

func (s *ClientDeliveryStore) EnsureDeliverySchema(ctx context.Context) error {
	return s.queue.EnsureDeliverySchema(ctx)
}

func (s *ClientDeliveryStore) SaveMessagePayload(ctx context.Context, record broker_store.MessagePayloadRecord) error {
	return s.payload.SaveMessagePayload(ctx, record)
}

func (s *ClientDeliveryStore) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	return s.payload.LoadMessagePayload(ctx, messageID)
}

func (s *ClientDeliveryStore) AppendDeliveryTask(ctx context.Context, task broker_store.DeliveryTask) (bool, error) {
	return s.queue.AppendDeliveryTask(ctx, task)
}

func (s *ClientDeliveryStore) DeliveryTaskExists(ctx context.Context, clientID string, messageID uuid.UUID) (bool, error) {
	return s.queue.DeliveryTaskExists(ctx, clientID, messageID)
}

func (s *ClientDeliveryStore) ReadDeliveryTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*broker_store.DeliveryTask, error) {
	return s.queue.ReadDeliveryTasks(ctx, clientID, lastTS, lastTaskID, limit)
}

func (s *ClientDeliveryStore) AdvanceDeliveryCursor(ctx context.Context, cursor broker_store.DeliveryCursor) (bool, error) {
	return s.queue.AdvanceDeliveryCursor(ctx, cursor)
}

func (s *ClientDeliveryStore) ReadDeliveryCursor(ctx context.Context, clientID string) (*broker_store.DeliveryCursor, error) {
	return s.queue.ReadDeliveryCursor(ctx, clientID)
}

func (s *ClientDeliveryStore) ResetClientDeliveryState(ctx context.Context, clientID string) error {
	return s.queue.ResetClientDeliveryState(ctx, clientID)
}

func (s *ClientDeliveryStore) DeleteClientSharedDeliveryTasks(ctx context.Context, clientID string, shareGroup string) error {
	deleter, ok := s.queue.(interface {
		DeleteClientSharedDeliveryTasks(context.Context, string, string) error
	})
	if !ok {
		return nil
	}
	return deleter.DeleteClientSharedDeliveryTasks(ctx, clientID, shareGroup)
}

func (s *ClientDeliveryStore) EnsureSchema(ctx context.Context) error {
	if s.shared == nil {
		return nil
	}
	return s.shared.EnsureSchema(ctx)
}

func (s *ClientDeliveryStore) AppendShareGroupTask(ctx context.Context, ts time.Time, task *sharedsubscription.ShareGroupTask) error {
	if s.shared == nil {
		return fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.AppendShareGroupTask(ctx, ts, task)
}

func (s *ClientDeliveryStore) ReadShareGroupTasks(ctx context.Context, shareGroup string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*sharedsubscription.ShareGroupTask, error) {
	if s.shared == nil {
		return nil, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.ReadShareGroupTasks(ctx, shareGroup, lastTS, lastTaskID, limit)
}

func (s *ClientDeliveryStore) MarkShareGroupTaskProcessed(ctx context.Context, taskID uuid.UUID, shareGroup string) error {
	if s.shared == nil {
		return fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.MarkShareGroupTaskProcessed(ctx, taskID, shareGroup)
}

func (s *ClientDeliveryStore) AtomicUpdateTaskStatus(
	ctx context.Context,
	taskID uuid.UUID,
	shareGroup string,
	oldStatus sharedsubscription.TaskStatus,
	newStatus sharedsubscription.TaskStatus,
) (bool, error) {
	if s.shared == nil {
		return false, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.AtomicUpdateTaskStatus(ctx, taskID, shareGroup, oldStatus, newStatus)
}

func (s *ClientDeliveryStore) GetUnAckedSharedSubscriptionTasks(ctx context.Context, clientID string, shareGroup string, lastTS time.Time, lastTaskID uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	if s.shared == nil {
		return nil, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.GetUnAckedSharedSubscriptionTasks(ctx, clientID, shareGroup, lastTS, lastTaskID)
}

func (s *ClientDeliveryStore) RollbackSharedSubscriptionTask(ctx context.Context, task *sharedsubscription.ShareGroupTask) error {
	if s.shared == nil {
		return fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.RollbackSharedSubscriptionTask(ctx, task)
}

func (s *ClientDeliveryStore) QueryProcessingTasksBefore(ctx context.Context, shareGroup string, beforeTime time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	if s.shared == nil {
		return nil, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.QueryProcessingTasksBefore(ctx, shareGroup, beforeTime)
}

func (s *ClientDeliveryStore) ReadShareGroupCursor(ctx context.Context, shareGroup string) (*sharedsubscription.ShareGroupCursor, error) {
	if s.shared == nil {
		return nil, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.ReadShareGroupCursor(ctx, shareGroup)
}

func (s *ClientDeliveryStore) AppendShareGroupCursor(ctx context.Context, shareGroup string, cursor *sharedsubscription.ShareGroupCursor) error {
	if s.shared == nil {
		return fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.AppendShareGroupCursor(ctx, shareGroup, cursor)
}

func (s *ClientDeliveryStore) QueryShareGroupTaskByMessageID(ctx context.Context, shareGroup string, messageID uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	if s.shared == nil {
		return nil, fmt.Errorf("shared subscription store is unavailable")
	}
	return s.shared.QueryShareGroupTaskByMessageID(ctx, shareGroup, messageID, statuses)
}

func (s *ClientDeliveryStore) DeliveryBacklogSummary(ctx context.Context) (broker_store.DeliveryBacklogSummary, error) {
	summaryStore, ok := s.queue.(broker_store.DeliveryBacklogSummaryStore)
	if !ok {
		return broker_store.DeliveryBacklogSummary{}, fmt.Errorf("%w: queue store does not support backlog summary", broker_store.ErrUnsupported)
	}
	return summaryStore.DeliveryBacklogSummary(ctx)
}

func (s *ClientDeliveryStore) Close() error {
	var err error
	if closer, ok := s.queue.(io.Closer); ok {
		err = errors.Join(err, closer.Close())
	}
	if closer, ok := s.payload.(io.Closer); ok {
		err = errors.Join(err, closer.Close())
	}
	return err
}
