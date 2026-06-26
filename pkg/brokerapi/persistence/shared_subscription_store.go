package persistence

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/google/uuid"
)

// SharedSubscriptionStore defines the interface for shared subscription storage operations.
// It is the metadata plane for shared subscription task queue and cursor management.
type SharedSubscriptionStore interface {
	// EnsureSchema creates required tables/structures if missing.
	EnsureSchema(ctx context.Context) error

	// AppendShareGroupTask appends a message to the shared subscription queue.
	AppendShareGroupTask(ctx context.Context, ts time.Time, task *sharedsubscription.ShareGroupTask) error

	// ReadShareGroupTasks reads unprocessed tasks from the shared subscription queue.
	ReadShareGroupTasks(ctx context.Context, shareGroup string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*sharedsubscription.ShareGroupTask, error)

	// MarkShareGroupTaskProcessed marks a task as processed.
	MarkShareGroupTaskProcessed(ctx context.Context, taskID uuid.UUID, shareGroup string) error

	// AtomicUpdateTaskStatus atomically updates task status using CAS operation.
	AtomicUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, shareGroup string, oldStatus, newStatus sharedsubscription.TaskStatus) (bool, error)

	// GetUnAckedSharedSubscriptionTasks gets unacked shared subscription tasks for a client.
	GetUnAckedSharedSubscriptionTasks(ctx context.Context, clientID string, shareGroup string, lastTS time.Time, lastTaskID uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error)

	// RollbackSharedSubscriptionTask rolls back a task to the shared queue.
	RollbackSharedSubscriptionTask(ctx context.Context, task *sharedsubscription.ShareGroupTask) error

	// QueryProcessingTasksBefore queries processing tasks that timed out before the given time.
	QueryProcessingTasksBefore(ctx context.Context, shareGroup string, beforeTime time.Time) ([]*sharedsubscription.ShareGroupTask, error)

	// ReadShareGroupCursor reads the consumption cursor for a shared subscription group.
	ReadShareGroupCursor(ctx context.Context, shareGroup string) (*sharedsubscription.ShareGroupCursor, error)

	// AppendShareGroupCursor updates the consumption cursor for a shared subscription group.
	AppendShareGroupCursor(ctx context.Context, shareGroup string, cursor *sharedsubscription.ShareGroupCursor) error

	// QueryShareGroupTaskByMessageID queries tasks by shareGroup and messageID with specific statuses.
	QueryShareGroupTaskByMessageID(ctx context.Context, shareGroup string, messageID uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error)
}

// CombinedSubscriptionStore supports both the client delivery pipeline and shared subscriptions.
type CombinedSubscriptionStore interface {
	ClientDeliveryStore
	SharedSubscriptionStore
}
