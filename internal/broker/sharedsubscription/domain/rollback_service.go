package domain

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/google/uuid"
)

type RollbackStore interface {
	GetUnAckedSharedSubscriptionTasks(ctx context.Context, clientID string, shareGroup string, lastTS time.Time, lastTaskID uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error)
	QueryShareGroupTaskByMessageID(ctx context.Context, shareGroup string, messageID uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error)
	AtomicUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, shareGroup string, oldStatus, newStatus sharedsubscription.TaskStatus) (bool, error)
	RollbackSharedSubscriptionTask(ctx context.Context, task *sharedsubscription.ShareGroupTask) error
	QueryProcessingTasksBefore(ctx context.Context, shareGroup string, beforeTime time.Time) ([]*sharedsubscription.ShareGroupTask, error)
}

type RollbackService struct {
	store       RollbackStore
	cursorStore delivery.CursorStore
	wakeFunc    func(shareGroup string)
}

func NewRollbackService(sharedStore RollbackStore, cursorStore delivery.CursorStore, wakeFunc func(shareGroup string)) *RollbackService {
	return &RollbackService{
		store:       sharedStore,
		cursorStore: cursorStore,
		wakeFunc:    wakeFunc,
	}
}

func (s *RollbackService) RollbackClientTasks(ctx context.Context, cmd RollbackCommand) {
	if s == nil || s.store == nil || cmd.ClientID == "" || cmd.ShareGroup == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cmd.Delay > 0 {
		timer := time.NewTimer(cmd.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	cursor, err := s.readCursor(ctx, cmd.ClientID)
	if err != nil {
		metric.RecordSharedRollback("client_offline", "error")
		logger.Logger.Warn().Err(err).Str("clientID", cmd.ClientID).Msg("failed to read delivery cursor")
		return
	}
	tasks, err := s.store.GetUnAckedSharedSubscriptionTasks(ctx, cmd.ClientID, cmd.ShareGroup, cursor.LastTS, cursor.LastTaskID)
	if err != nil {
		metric.RecordSharedRollback("client_offline", "error")
		logger.Logger.Warn().Err(err).Str("clientID", cmd.ClientID).Str("shareGroup", cmd.ShareGroup).Msg("failed to query unacked shared tasks")
		return
	}

	rolledBackAny := false
	for _, task := range tasks {
		if ctx.Err() != nil {
			return
		}
		if task == nil {
			continue
		}
		if !s.prepareTaskRollback(ctx, cmd.ShareGroup, task) {
			metric.RecordSharedRollback("duplicate_guard", "skip")
			metric.RecordDuplicateDelivery("shared", "runner")
			continue
		}
		if err := s.store.RollbackSharedSubscriptionTask(ctx, task); err != nil {
			metric.RecordSharedRollback("client_offline", "error")
			logger.Logger.Warn().Err(err).Str("taskID", task.TaskID.String()).Msg("failed to rollback shared task")
			continue
		}
		rolledBackAny = true
		metric.RecordSharedRollback("client_offline", "success")
		logger.Logger.Info().Str("taskID", task.TaskID.String()).Str("shareGroup", cmd.ShareGroup).Str("clientID", cmd.ClientID).Msg("rolled back shared task")
	}

	if rolledBackAny {
		if deleter, ok := s.cursorStore.(delivery.SharedClientTaskDeleter); ok {
			if err := deleter.DeleteClientSharedTasks(ctx, cmd.ClientID, cmd.ShareGroup); err != nil {
				logger.Logger.Warn().Err(err).Str("shareGroup", cmd.ShareGroup).Str("clientID", cmd.ClientID).Msg("failed to delete stale shared client tasks")
			}
		}
		if s.wakeFunc != nil {
			s.wakeFunc(cmd.ShareGroup)
		}
	}
}

func (s *RollbackService) RequeueTimeoutTasks(ctx context.Context, shareGroup string, cutoffTime time.Time) {
	if s == nil || s.store == nil || shareGroup == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tasks, err := s.store.QueryProcessingTasksBefore(ctx, shareGroup, cutoffTime)
	if err != nil {
		metric.RecordProcessingTimeout("shared", "error")
		metric.RecordSharedRollback("processing_timeout", "error")
		logger.Logger.Warn().Err(err).Str("shareGroup", shareGroup).Msg("failed to query timeout shared tasks")
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if !CanTransitionTaskStatus(sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusPending) {
			metric.RecordProcessingTimeout("shared", "skip")
			metric.RecordSharedRollback("processing_timeout", "skip")
			continue
		}
		updated, updateErr := s.store.AtomicUpdateTaskStatus(ctx, task.TaskID, shareGroup, sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusPending)
		if updateErr != nil {
			metric.RecordProcessingTimeout("shared", "error")
			metric.RecordSharedRollback("processing_timeout", "error")
			logger.Logger.Warn().Err(updateErr).Str("taskID", task.TaskID.String()).Msg("failed to revert timeout shared task")
			continue
		}
		if updated {
			metric.RecordProcessingTimeout("shared", "success")
			metric.RecordSharedRollback("processing_timeout", "success")
			logger.Logger.Info().Str("taskID", task.TaskID.String()).Str("shareGroup", shareGroup).Msg("reverted timeout shared task to pending")
		} else {
			metric.RecordProcessingTimeout("shared", "skip")
			metric.RecordSharedRollback("processing_timeout", "skip")
		}
	}
}

func (s *RollbackService) prepareTaskRollback(ctx context.Context, shareGroup string, task *sharedsubscription.ShareGroupTask) bool {
	processingTask, err := s.store.QueryShareGroupTaskByMessageID(ctx, shareGroup, task.MessageID, []sharedsubscription.TaskStatus{
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusProcessing,
	})
	if err != nil {
		logger.Logger.Warn().Err(err).Str("taskID", task.TaskID.String()).Msg("failed to query shared task before rollback")
		return false
	}
	if processingTask == nil {
		return true
	}
	if processingTask.Status != sharedsubscription.TaskStatusProcessing {
		return true
	}

	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case <-ctx.Done():
		timer.Stop()
		return false
	case <-timer.C:
	}
	completedTask, _ := s.store.QueryShareGroupTaskByMessageID(ctx, shareGroup, task.MessageID, []sharedsubscription.TaskStatus{
		sharedsubscription.TaskStatusCompleted,
	})
	if completedTask != nil && completedTask.Status == sharedsubscription.TaskStatusCompleted {
		return false
	}

	if !CanTransitionTaskStatus(sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusRolledBack) {
		return false
	}
	updated, err := s.store.AtomicUpdateTaskStatus(ctx, processingTask.TaskID, shareGroup, sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusRolledBack)
	return err == nil && updated
}

func (s *RollbackService) readCursor(ctx context.Context, clientID string) (*store.DeliveryCursor, error) {
	if s.cursorStore == nil {
		return &store.DeliveryCursor{ClientID: clientID, LastTS: time.Time{}, LastTaskID: uuid.Nil}, nil
	}
	cursor, err := s.cursorStore.ReadCursor(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if cursor == nil {
		return &store.DeliveryCursor{ClientID: clientID, LastTS: time.Time{}, LastTaskID: uuid.Nil}, nil
	}
	return cursor, nil
}
