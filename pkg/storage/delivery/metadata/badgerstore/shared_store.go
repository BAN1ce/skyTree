package badgerstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

const sharedDeliveryReadBatchSize = 200

var (
	_ brokerstore.SharedSubscriptionStore = (*LocalDeliveryQueueStore)(nil)
)

func (s *LocalDeliveryQueueStore) EnsureSchema(ctx context.Context) error {
	return s.EnsureDeliverySchema(ctx)
}

func (s *LocalDeliveryQueueStore) AppendShareGroupTask(
	ctx context.Context,
	ts time.Time,
	task *sharedsubscription.ShareGroupTask,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := appendShareTaskRequestFromTask(ts, task)
	if err != nil {
		return err
	}
	b, err := marshalUpdate(opAppendShareTask, req)
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) ReadShareGroupTasks(
	ctx context.Context,
	shareGroup string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
	limit int,
) ([]*sharedsubscription.ShareGroupTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&readShareTasksQuery{
		ShareGroup:     shareGroup,
		LastTSUnixNano: lastTS.UnixNano(),
		LastTaskID:     lastTaskID,
		Limit:          limit,
	})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.([]*sharedsubscription.ShareGroupTask); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) MarkShareGroupTaskProcessed(
	ctx context.Context,
	taskID uuid.UUID,
	shareGroup string,
) error {
	return markShareGroupTaskProcessed(ctx, s, taskID, shareGroup)
}

func (s *LocalDeliveryQueueStore) AtomicUpdateTaskStatus(
	ctx context.Context,
	taskID uuid.UUID,
	shareGroup string,
	oldStatus sharedsubscription.TaskStatus,
	newStatus sharedsubscription.TaskStatus,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	b, err := marshalUpdate(opUpdateShareTaskStatus, updateShareTaskStatusRequest{
		TaskID:     taskID.String(),
		ShareGroup: shareGroup,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		NowNano:    time.Now().UnixNano(),
	})
	if err != nil {
		return false, err
	}
	result, err := s.sm.Update(b)
	return result.Value == 1, err
}

func (s *LocalDeliveryQueueStore) GetUnAckedSharedSubscriptionTasks(
	ctx context.Context,
	clientID string,
	shareGroup string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
) ([]*sharedsubscription.ShareGroupTask, error) {
	return readUnackedSharedSubscriptionTasks(ctx, s, clientID, shareGroup, lastTS, lastTaskID)
}

func (s *LocalDeliveryQueueStore) RollbackSharedSubscriptionTask(
	ctx context.Context,
	task *sharedsubscription.ShareGroupTask,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := appendShareTaskRequestFromTask(time.Now(), task)
	if err != nil {
		return err
	}
	b, err := marshalUpdate(opRollbackShareGroupTask, req)
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) QueryProcessingTasksBefore(
	ctx context.Context,
	shareGroup string,
	beforeTime time.Time,
) ([]*sharedsubscription.ShareGroupTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&queryProcessingShareTasksBeforeQuery{
		ShareGroup: shareGroup,
		BeforeNano: beforeTime.UnixNano(),
	})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.([]*sharedsubscription.ShareGroupTask); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) ReadShareGroupCursor(
	ctx context.Context,
	shareGroup string,
) (*sharedsubscription.ShareGroupCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&readShareCursorQuery{ShareGroup: shareGroup})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.(*sharedsubscription.ShareGroupCursor); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) AppendShareGroupCursor(
	ctx context.Context,
	shareGroup string,
	cursor *sharedsubscription.ShareGroupCursor,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := appendShareGroupCursorRequestFromCursor(shareGroup, cursor)
	if err != nil {
		return err
	}
	b, err := marshalUpdate(opAppendShareGroupCursor, req)
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) QueryShareGroupTaskByMessageID(
	ctx context.Context,
	shareGroup string,
	messageID uuid.UUID,
	statuses []sharedsubscription.TaskStatus,
) (*sharedsubscription.ShareGroupTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&queryShareTaskByMessageIDQuery{
		ShareGroup: shareGroup,
		MessageID:  messageID,
		Statuses:   statuses,
	})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.(*sharedsubscription.ShareGroupTask); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}


type deliveryTaskReader interface {
	ReadDeliveryTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*brokerstore.DeliveryTask, error)
}

type sharedStatusUpdater interface {
	AtomicUpdateTaskStatus(
		ctx context.Context,
		taskID uuid.UUID,
		shareGroup string,
		oldStatus sharedsubscription.TaskStatus,
		newStatus sharedsubscription.TaskStatus,
	) (bool, error)
}

func appendShareTaskRequestFromTask(ts time.Time, task *sharedsubscription.ShareGroupTask) (appendShareTaskRequest, error) {
	if task == nil {
		return appendShareTaskRequest{}, fmt.Errorf("shared subscription task is nil")
	}
	if task.TaskID == uuid.Nil {
		return appendShareTaskRequest{}, fmt.Errorf("shared subscription taskID is empty")
	}
	if task.ShareGroup == "" {
		return appendShareTaskRequest{}, fmt.Errorf("shareGroup is empty")
	}
	if task.MessageID == uuid.Nil {
		return appendShareTaskRequest{}, fmt.Errorf("messageID is empty")
	}
	status := task.Status
	if status == "" {
		status = sharedsubscription.TaskStatusPending
	}
	if ts.IsZero() {
		ts = task.Timestamp
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	return appendShareTaskRequest{
		TSUnixNano:      ts.UnixNano(),
		TaskID:          task.TaskID.String(),
		ShareGroup:      task.ShareGroup,
		TopicFilter:     task.TopicFilter,
		MessageID:       task.MessageID.String(),
		DeliveryQoS:     task.DeliveryQoS,
		PublishQoS:      task.PublishQoS,
		PublisherClient: task.PublisherClient,
		SubscriptionIDs: task.SubscriptionIDs,
		WinnerNoLocal:   task.WinnerNoLocal,
		WinnerRAP:       task.WinnerRAP,
		Status:          status,
		ProcessedByNode: task.ProcessedByNode,
		ProcessedAtNano: task.ProcessedAt.UnixNano(),
		RollbackReason:  task.RollbackReason,
	}, nil
}

func appendShareGroupCursorRequestFromCursor(
	shareGroup string,
	cursor *sharedsubscription.ShareGroupCursor,
) (appendShareGroupCursorRequest, error) {
	if cursor == nil {
		return appendShareGroupCursorRequest{}, fmt.Errorf("share group cursor is nil")
	}
	if shareGroup == "" {
		shareGroup = cursor.ShareGroup
	}
	if shareGroup == "" {
		return appendShareGroupCursorRequest{}, fmt.Errorf("shareGroup is empty")
	}
	return appendShareGroupCursorRequest{
		ShareGroup:          shareGroup,
		LastProcessedTSNano: cursor.LastProcessedTS.UnixNano(),
		LastProcessedTaskID: cursor.LastProcessedTaskID.String(),
		LeaderNodeID:        cursor.LeaderNodeID,
		LastRenewalNano:     cursor.LastRenewal.UnixNano(),
	}, nil
}

func markShareGroupTaskProcessed(
	ctx context.Context,
	updater sharedStatusUpdater,
	taskID uuid.UUID,
	shareGroup string,
) error {
	for _, oldStatus := range []sharedsubscription.TaskStatus{
		sharedsubscription.TaskStatusProcessing,
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusRolledBack,
	} {
		updated, err := updater.AtomicUpdateTaskStatus(
			ctx,
			taskID,
			shareGroup,
			oldStatus,
			sharedsubscription.TaskStatusCompleted,
		)
		if err != nil || updated {
			return err
		}
	}
	return nil
}

func readUnackedSharedSubscriptionTasks(
	ctx context.Context,
	reader deliveryTaskReader,
	clientID string,
	shareGroup string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
) ([]*sharedsubscription.ShareGroupTask, error) {
	out := make([]*sharedsubscription.ShareGroupTask, 0)
	for {
		tasks, err := reader.ReadDeliveryTasks(ctx, clientID, lastTS, lastTaskID, sharedDeliveryReadBatchSize)
		if err != nil {
			return nil, err
		}
		if len(tasks) == 0 {
			return out, nil
		}
		for _, task := range tasks {
			if task == nil {
				continue
			}
			lastTS = task.TS
			lastTaskID = task.TaskID
			if task.ShareGroup != shareGroup || task.SharedTaskID == uuid.Nil {
				continue
			}
			out = append(out, shareGroupTaskFromDeliveryTask(task))
		}
		if len(tasks) < sharedDeliveryReadBatchSize {
			return out, nil
		}
	}
}

func checkDeliveryTaskExists(
	ctx context.Context,
	reader deliveryTaskReader,
	clientID string,
	messageID uuid.UUID,
) (bool, error) {
	var (
		lastTS     time.Time
		lastTaskID uuid.UUID
	)
	for {
		tasks, err := reader.ReadDeliveryTasks(ctx, clientID, lastTS, lastTaskID, sharedDeliveryReadBatchSize)
		if err != nil {
			return false, err
		}
		if len(tasks) == 0 {
			return false, nil
		}
		for _, task := range tasks {
			if task == nil {
				continue
			}
			lastTS = task.TS
			lastTaskID = task.TaskID
			if task.MessageID == messageID {
				return true, nil
			}
		}
		if len(tasks) < sharedDeliveryReadBatchSize {
			return false, nil
		}
	}
}

func shareGroupTaskFromDeliveryTask(task *brokerstore.DeliveryTask) *sharedsubscription.ShareGroupTask {
	taskID := task.SharedTaskID
	if taskID == uuid.Nil {
		taskID = task.TaskID
	}
	return &sharedsubscription.ShareGroupTask{
		TaskID:          taskID,
		ShareGroup:      task.ShareGroup,
		MessageID:       task.MessageID,
		DeliveryQoS:     task.DeliveryQoS,
		SubscriptionIDs: subscriptionIDsJSON(task.SubscriptionIDs),
		WinnerNoLocal:   task.NoLocal,
		WinnerRAP:       task.RetainAsPublished,
		Timestamp:       task.TS,
	}
}

func subscriptionIDsJSON(ids []int32) string {
	if len(ids) == 0 {
		return ""
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(b)
}
