package memory

import (
	"context"
	"time"

	willdelay "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay/internal/state"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

// Center is an internal in-memory will-delay store for tests and lightweight local wiring.
type Center struct {
	core *state.Core
}

var _ willdelay.Center = (*Center)(nil)
var _ willdelay.OwnerTaskDeleter = (*Center)(nil)

// New creates a new in-memory Center.
func New() *Center {
	core := state.NewCore(1*time.Second, 3600)
	return &Center{
		core: core,
	}
}

// AddTask adds a will-delay task.
func (m *Center) AddTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error {
	_ = ctx
	return m.core.AddTask(task)
}

// DeleteTask deletes all will-delay tasks for a client.
func (m *Center) DeleteTask(ctx context.Context, clientID string) error {
	_ = ctx
	m.core.DeleteTask(clientID)
	return nil
}

// DeleteTaskByOwner deletes one owner-token task variant.
func (m *Center) DeleteTaskByOwner(ctx context.Context, clientID, ownerToken string) error {
	_ = ctx
	m.core.DeleteTaskByOwner(clientID, ownerToken)
	return nil
}

// GetDueTasks returns tasks whose scheduled publish time is at or before nowUnixMicro.
func (m *Center) GetDueTasks(
	ctx context.Context,
	nowUnixMicro int64,
) ([]*proto_will_delay.WillDelayTask, error) {
	_ = ctx
	return m.core.GetDueTasks(nowUnixMicro), nil
}
