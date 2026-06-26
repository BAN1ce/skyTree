package raft

import (
	"context"
	"fmt"

	willdelay "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay/internal/state"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"google.golang.org/protobuf/proto"
)

// Cluster proxies will-delay operations to the Raft-backed state machine.
type Cluster struct {
	client      cluster.Client
	localNodeID uint64
}

var _ willdelay.Center = (*Cluster)(nil)
var _ willdelay.OwnerTaskDeleter = (*Cluster)(nil)

// NewCluster creates a new Raft-backed will-delay center.
func NewCluster(localNodeID uint64, client cluster.Client) *Cluster {
	return &Cluster{
		client:      client,
		localNodeID: localNodeID,
	}
}

// AddTask adds a will-delay task.
func (c *Cluster) AddTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error {
	if err := state.ValidateTaskForAdd(task); err != nil {
		return err
	}
	req := &proto_will_delay.WillDelayRequest{
		Type: proto_will_delay.WillDelayRequestType_ADD_TASK,
		Task: proto.Clone(task).(*proto_will_delay.WillDelayTask),
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.client.Write(ctx, data)
	return err
}

// DeleteTask deletes all will-delay tasks for a client.
func (c *Cluster) DeleteTask(ctx context.Context, clientID string) error {
	req := &proto_will_delay.WillDelayRequest{
		Type:     proto_will_delay.WillDelayRequestType_DELETE_TASK,
		ClientID: &clientID,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.client.Write(ctx, data)
	return err
}

// DeleteTaskByOwner deletes one owner-token task variant.
func (c *Cluster) DeleteTaskByOwner(ctx context.Context, clientID, ownerToken string) error {
	req := &proto_will_delay.WillDelayRequest{
		Type:     proto_will_delay.WillDelayRequestType_DELETE_TASK,
		ClientID: &clientID,
		Task: &proto_will_delay.WillDelayTask{
			ClientID:   clientID,
			OwnerToken: ownerToken,
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.client.Write(ctx, data)
	return err
}

// GetDueTasks returns tasks whose scheduled publish time is at or before nowUnixMicro.
func (c *Cluster) GetDueTasks(
	ctx context.Context,
	nowUnixMicro int64,
) ([]*proto_will_delay.WillDelayTask, error) {
	req := &proto_will_delay.WillDelayRequest{
		Type:        proto_will_delay.WillDelayRequestType_GET_DUE_TASKS,
		CurrentTime: &nowUnixMicro,
	}

	result, err := c.client.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	response, ok := result.(*proto_will_delay.WillDelayResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", result)
	}

	return response.GetDueTasks(), nil
}
