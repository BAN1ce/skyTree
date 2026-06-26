package wal

import (
	"context"
	"fmt"
	"time"

	willdelay "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay/internal/state"
	wdsm "github.com/BAN1ce/skyTree/internal/broker/willdelay/statemachine"
	"github.com/BAN1ce/skyTree/internal/localstate/walsm"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"google.golang.org/protobuf/proto"
)

// LocalWALCenter stores will-delay state in a single-node WAL and snapshots.
type LocalWALCenter struct {
	engine *walsm.Engine
}

var _ willdelay.Center = (*LocalWALCenter)(nil)
var _ willdelay.OwnerTaskDeleter = (*LocalWALCenter)(nil)

// NewLocalWALCenter creates a single-node persistent will-delay center.
func NewLocalWALCenter(
	baseDir string,
	snapshotInterval time.Duration,
	snapshotEntries uint64,
) (*LocalWALCenter, error) {
	engine, err := walsm.NewEngine(wdsm.New(), walsm.Options{
		Name:             "will_delay_center",
		BaseDir:          baseDir,
		SnapshotEntries:  snapshotEntries,
		SnapshotInterval: snapshotInterval,
	})
	if err != nil {
		return nil, err
	}
	return &LocalWALCenter{engine: engine}, nil
}

// Close closes the WAL engine.
func (c *LocalWALCenter) Close() error {
	if c == nil || c.engine == nil {
		return nil
	}
	return c.engine.Close()
}

// AddTask adds a will-delay task.
func (c *LocalWALCenter) AddTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error {
	if err := state.ValidateTaskForAdd(task); err != nil {
		return err
	}
	req := &proto_will_delay.WillDelayRequest{
		Type: proto_will_delay.WillDelayRequestType_ADD_TASK,
		Task: proto.Clone(task).(*proto_will_delay.WillDelayTask),
	}
	return c.write(ctx, req)
}

// DeleteTask deletes all will-delay tasks for a client.
func (c *LocalWALCenter) DeleteTask(ctx context.Context, clientID string) error {
	req := &proto_will_delay.WillDelayRequest{
		Type:     proto_will_delay.WillDelayRequestType_DELETE_TASK,
		ClientID: &clientID,
	}
	return c.write(ctx, req)
}

// DeleteTaskByOwner deletes one owner-token task variant.
func (c *LocalWALCenter) DeleteTaskByOwner(ctx context.Context, clientID, ownerToken string) error {
	req := &proto_will_delay.WillDelayRequest{
		Type:     proto_will_delay.WillDelayRequestType_DELETE_TASK,
		ClientID: &clientID,
		Task: &proto_will_delay.WillDelayTask{
			ClientID:   clientID,
			OwnerToken: ownerToken,
		},
	}
	return c.write(ctx, req)
}

// GetDueTasks returns tasks whose scheduled publish time is at or before nowUnixMicro.
func (c *LocalWALCenter) GetDueTasks(
	ctx context.Context,
	nowUnixMicro int64,
) ([]*proto_will_delay.WillDelayTask, error) {
	_ = ctx

	req := &proto_will_delay.WillDelayRequest{
		Type:        proto_will_delay.WillDelayRequestType_GET_DUE_TASKS,
		CurrentTime: &nowUnixMicro,
	}
	out, err := c.engine.Read(req)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto_will_delay.WillDelayResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	return resp.GetDueTasks(), nil
}

func (c *LocalWALCenter) write(ctx context.Context, req *proto_will_delay.WillDelayRequest) error {
	_ = ctx

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(data)
	return err
}
