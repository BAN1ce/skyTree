package cluster

import (
	"context"

	"github.com/BAN1ce/skyTree/config"
)

type NodeMeta struct {
	config.Cluster
}

type State interface {
	AddNode(ctx context.Context, request *NodeMeta) error
	RemoveNode(ctx context.Context, nodeID uint64) error
	ListNode(ctx context.Context) ([]*NodeMeta, error)
}
