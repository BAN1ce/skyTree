package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

// appendSharedDeliveryTasks 将共享订阅目标写入 share group 队列，并唤醒对应消费协程。
func (b *Broker) appendSharedDeliveryTasks(
	ctx context.Context,
	now time.Time,
	topic string,
	messageID uuid.UUID,
	shareTasks []delivery.ShareGroupTask,
) error {
	if len(shareTasks) == 0 {
		return nil
	}
	if b.shared.store == nil {
		return errors.New("shared subscription store is nil")
	}
	var appendErrs []error
	for _, shareTask := range shareTasks {
		if shareTask.ShareGroup == "" {
			continue
		}
		existing, err := b.shared.store.QueryShareGroupTaskByMessageID(ctx, shareTask.ShareGroup, messageID, activeShareTaskStatuses())
		if err != nil {
			logger.Logger.Error().Err(err).Str("shareGroup", shareTask.ShareGroup).Str("topic", topic).Msg("query share group task failed")
			appendErrs = append(appendErrs, fmt.Errorf("share group %s: %w", shareTask.ShareGroup, err))
			continue
		}
		if existing != nil {
			if err := b.notifyShareTaskAppended(ctx, existing); err != nil {
				appendErrs = append(appendErrs, fmt.Errorf("notify share group %s: %w", shareTask.ShareGroup, err))
			}
			continue
		}
		task := newPendingShareGroupTask(now, messageID, shareTask)
		if err := b.shared.store.AppendShareGroupTask(ctx, now, task); err != nil {
			logger.Logger.Error().Err(err).Str("shareGroup", shareTask.ShareGroup).Str("topic", topic).Msg("append share group task failed")
			appendErrs = append(appendErrs, fmt.Errorf("share group %s: %w", shareTask.ShareGroup, err))
			continue
		}
		if err := b.notifyShareTaskAppended(ctx, task); err != nil {
			appendErrs = append(appendErrs, fmt.Errorf("notify share group %s: %w", task.ShareGroup, err))
		}
	}
	if len(appendErrs) > 0 {
		return errors.Join(appendErrs...)
	}
	return nil
}

func (b *Broker) notifyShareTaskAppended(ctx context.Context, task *sharedsubscription.ShareGroupTask) error {
	if b == nil || task == nil {
		return nil
	}
	notifier := b.shared.taskNotifier()
	if notifier == nil {
		return nil
	}
	return notifier.NotifyTaskAppended(ctx, shared_manager.TaskAppendedEvent{
		ShareGroup:  task.ShareGroup,
		TopicFilter: task.TopicFilter,
		TaskID:      task.TaskID,
	})
}

func activeShareTaskStatuses() []sharedsubscription.TaskStatus {
	return []sharedsubscription.TaskStatus{
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusProcessing,
	}
}

// newPendingShareGroupTask 将路由结果转换为共享订阅队列中的 pending task。
func newPendingShareGroupTask(now time.Time, messageID uuid.UUID, shareTask delivery.ShareGroupTask) *sharedsubscription.ShareGroupTask {
	return &sharedsubscription.ShareGroupTask{
		TaskID:          uuid.New(),
		ShareGroup:      shareTask.ShareGroup,
		TopicFilter:     shareTask.TopicFilter,
		MessageID:       messageID,
		DeliveryQoS:     shareTask.DeliveryQoS,
		PublishQoS:      shareTask.PublishQoS,
		PublisherClient: shareTask.PublisherClient,
		SubscriptionIDs: shareTask.SubscriptionIDs,
		WinnerNoLocal:   shareTask.WinnerNoLocal,
		WinnerRAP:       shareTask.WinnerRAP,
		Status:          sharedsubscription.TaskStatusPending,
		Timestamp:       now,
	}
}
