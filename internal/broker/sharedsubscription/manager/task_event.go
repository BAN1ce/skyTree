package manager

import (
	"context"
	"fmt"

	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/google/uuid"
)

type TaskAppendedEvent struct {
	ShareGroup  string
	TopicFilter string
	TaskID      uuid.UUID
}

func (e TaskAppendedEvent) Validate() error {
	if e.ShareGroup == "" {
		return fmt.Errorf("shareGroup is empty")
	}
	if e.TopicFilter == "" {
		return fmt.Errorf("topicFilter is empty")
	}
	if e.TaskID == uuid.Nil {
		return fmt.Errorf("taskID is empty")
	}
	return nil
}

// NotifyTaskAppended is the domain entrypoint after a shared subscription task is persisted.
func (m *SharedSubscriptionManager) NotifyTaskAppended(ctx context.Context, event TaskAppendedEvent) error {
	if m == nil {
		return nil
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := m.ensureConsumer(ctx, event.ShareGroup, event.TopicFilter); err != nil {
		return fmt.Errorf("ensure shared consumer: %w", err)
	}
	m.wakeConsumer(event.ShareGroup)
	if err := m.broadcastSharedWake(ctx, event); err != nil {
		return fmt.Errorf("broadcast shared wake: %w", err)
	}
	return nil
}

// HandleRemoteTaskAppended wakes the local shared consumer for a task announced by a peer node.
func (m *SharedSubscriptionManager) HandleRemoteTaskAppended(ctx context.Context, event TaskAppendedEvent) error {
	if m == nil {
		return nil
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := m.ensureConsumer(ctx, event.ShareGroup, event.TopicFilter); err != nil {
		return fmt.Errorf("ensure remote shared consumer: %w", err)
	}
	m.wakeConsumer(event.ShareGroup)
	return nil
}

func (m *SharedSubscriptionManager) broadcastSharedWake(ctx context.Context, event TaskAppendedEvent) error {
	if m == nil || m.deliveryEvent == nil {
		return nil
	}
	return m.deliveryEvent.NotifySharedWake(ctx, delivery_notify.SharedWakePayload{
		ShareGroup:  event.ShareGroup,
		TopicFilter: event.TopicFilter,
		TaskID:      event.TaskID.String(),
	})
}
