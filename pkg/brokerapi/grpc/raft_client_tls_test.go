package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg/cluster"
)

func TestRaftGRPCClientRejectsInsecureWhenNotAllowed(t *testing.T) {
	client := NewRaftGRPCClient(1, nil, config.TLS{Enabled: false}, false)
	if client.credentialErr == nil {
		t.Fatal("expected credential error when tls disabled and allow_insecure=false")
	}
}

func TestRaftGRPCClientAllowsInsecureWhenExplicitlyEnabled(t *testing.T) {
	client := NewRaftGRPCClient(1, nil, config.TLS{Enabled: false}, true)
	if client.credentialErr != nil {
		t.Fatalf("expected insecure grpc client to be allowed, got error: %v", client.credentialErr)
	}
}

type blockingClusterState struct{}

func (blockingClusterState) AddNode(context.Context, *cluster.NodeMeta) error {
	return nil
}

func (blockingClusterState) RemoveNode(context.Context, uint64) error {
	return nil
}

func (blockingClusterState) ListNode(ctx context.Context) ([]*cluster.NodeMeta, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRaftGRPCClientNotifyClientDeliveryUsesPerCallTimeout(t *testing.T) {
	client := NewRaftGRPCClient(1, blockingClusterState{}, config.TLS{Enabled: false}, true)
	client.rpcTimeout = 20 * time.Millisecond

	start := time.Now()
	err := client.NotifyClientDelivery(context.Background(), 2, "sensors/1", []string{"client-a"}, 1, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NotifyClientDelivery error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("NotifyClientDelivery took %s, want bounded by per-call timeout", elapsed)
	}
}
