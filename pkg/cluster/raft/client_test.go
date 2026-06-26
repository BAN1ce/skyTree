package raft

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/lni/dragonboat/v3"
	dbconfig "github.com/lni/dragonboat/v3/config"
)

func TestClientWriteReturnsErrorWhenClusterNotReady(t *testing.T) {
	client := NewClient(ClusterIDKeyStore, nil)
	_, err := client.Write(context.Background(), []byte("payload"))
	if err == nil {
		t.Fatal("expected error when raft cluster is not ready")
	}
}

func TestClientReadReturnsErrorWhenClusterNotReady(t *testing.T) {
	client := NewClient(ClusterIDKeyStore, nil)
	_, err := client.Read(context.Background(), "query")
	if err == nil {
		t.Fatal("expected error when raft cluster is not ready")
	}
}

func TestClientReadWithNilContextReturnsErrorWhenClusterNotReady(t *testing.T) {
	client := NewClient(ClusterIDKeyStore, nil)
	_, err := client.Read(nil, "query")
	if err == nil {
		t.Fatal("expected error when raft cluster is not ready")
	}
}

func TestClientWriteReturnsErrorWhenProposeFails(t *testing.T) {
	ensureTestLogger(t)
	nh := newTestNodeHost(t)
	defer nh.Stop()

	client := NewClient(ClusterIDKeyStore, &Cluster{node: nh})
	_, err := client.Write(context.Background(), []byte("payload"))
	if err == nil {
		t.Fatal("expected error when propose fails")
	}
}

func TestClientWriteHonorsCanceledContext(t *testing.T) {
	ensureTestLogger(t)
	nh := newTestNodeHost(t)
	defer nh.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(ClusterIDKeyStore, &Cluster{node: nh})
	_, err := client.Write(ctx, []byte("payload"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func ensureTestLogger(t *testing.T) {
	t.Helper()
	if logger.Logger == nil {
		logger.LoadForTest()
	}
}

func newTestNodeHost(t *testing.T) *dragonboat.NodeHost {
	t.Helper()

	walDir := filepath.Join(t.TempDir(), "wal")
	nodeHostDir := filepath.Join(t.TempDir(), "nodehost")
	nh, err := dragonboat.NewNodeHost(dbconfig.NodeHostConfig{
		WALDir:         walDir,
		NodeHostDir:    nodeHostDir,
		RTTMillisecond: 10,
		RaftAddress:    testTCPAddress(t),
	})
	if err != nil {
		t.Fatalf("create node host failed: %v", err)
	}
	return nh
}

func testTCPAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}
