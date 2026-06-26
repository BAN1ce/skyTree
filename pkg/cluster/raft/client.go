package raft

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/lni/dragonboat/v3/statemachine"
)

type Client struct {
	clusterID uint64
	cluster   *Cluster
	// default 5s
	writeTimeout time.Duration
	// default 5s
	readTimeout time.Duration
}

type ClientOption func(*Client)

func WithTimeout(read, write time.Duration) ClientOption {
	return func(client *Client) {
		client.readTimeout = read
		client.writeTimeout = write
	}
}

func NewClient(clusterID uint64, cluster *Cluster, options ...ClientOption) *Client {
	c := &Client{
		clusterID: clusterID,
		cluster:   cluster,
	}

	for _, option := range options {
		option(c)
	}
	if c.writeTimeout == 0 {
		c.writeTimeout = 5 * time.Second
	}

	if c.readTimeout == 0 {
		c.readTimeout = 5 * time.Second
	}
	return c
}

func (c *Client) Write(ctx context.Context, data []byte) (result statemachine.Result, err error) {
	var (
		cancel    context.CancelFunc
		startTime = time.Now()
	)
	defer func() {
		if cancel != nil {
			cancel()
		}
		metric.RecordRaftRequest(ClusterName(c.clusterID), c.clusterID, "write", err, time.Since(startTime), len(data))
	}()

	if len(data) == 0 {
		return statemachine.Result{}, fmt.Errorf("raft write data is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return statemachine.Result{}, err
	}
	if c.cluster == nil || c.cluster.node == nil {
		return statemachine.Result{}, fmt.Errorf("raft cluster is not ready for clusterID %d", c.clusterID)
	}

	ctx, cancel = context.WithTimeout(ctx, c.writeTimeout)

	return c.cluster.node.SyncPropose(ctx, c.cluster.node.GetNoOPSession(c.clusterID), data)
}

func (c *Client) Read(ctx context.Context, query interface{}) (resp interface{}, err error) {
	var (
		timeStart = time.Now()
		cancel    context.CancelFunc
	)
	defer func() {
		if cancel != nil {
			cancel()
		}
		metric.RecordRaftRequest(ClusterName(c.clusterID), c.clusterID, "read", err, time.Since(timeStart), 0)
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.cluster == nil || c.cluster.node == nil {
		return nil, fmt.Errorf("raft cluster is not ready for clusterID %d", c.clusterID)
	}

	ctx, cancel = context.WithTimeout(ctx, c.readTimeout)

	return c.cluster.node.SyncRead(ctx, c.clusterID, query)
}

func (c *Client) GetNodeID() uint64 {
	// clusterID identifies a Raft group, while NodeHost ID identifies the broker node.
	if c.cluster == nil || c.cluster.node == nil {
		return 0
	}
	// NodeHost.ID() returns a string, usually a base-10 integer.
	idStr := c.cluster.node.ID()
	if idStr == "" {
		return 0
	}
	// Best-effort parse; if parsing fails, return 0 to avoid panics in callers.
	// (Callers should treat 0 as "unknown node".)
	v, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func (c *Client) ReadTimeout() time.Duration {
	if c == nil {
		return 0
	}
	return c.readTimeout
}

func (c *Client) WriteTimeout() time.Duration {
	if c == nil {
		return 0
	}
	return c.writeTimeout
}
