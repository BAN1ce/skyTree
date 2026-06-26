package willdelay

import (
	"context"

	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

// Center defines the will-delay task storage contract used by broker runtime.
type Center interface {
	AddTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error
	DeleteTask(ctx context.Context, clientID string) error
	GetDueTasks(ctx context.Context, nowUnixMicro int64) ([]*proto_will_delay.WillDelayTask, error)
}

// OwnerTaskDeleter deletes one owner-token variant without touching newer owners.
type OwnerTaskDeleter interface {
	DeleteTaskByOwner(ctx context.Context, clientID, ownerToken string) error
}
