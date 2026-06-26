package clusterruntime

import (
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

// TestNewRaftClusterUsesJoinConfig 验证 raft cluster 会应用 join 配置。
func TestNewRaftClusterUsesJoinConfig(t *testing.T) {
	cluster := NewRaftCluster(config.Cluster{Join: true})
	if !cluster.IsJoinMode() {
		t.Fatal("expected join mode to be enabled")
	}

	cluster = NewRaftCluster(config.Cluster{Join: false})
	if cluster.IsJoinMode() {
		t.Fatal("expected join mode to be disabled")
	}
}

// TestNewBusinessRaftClientUsesConfiguredWriteTimeout 验证业务 raft client 使用配置的写超时。
func TestNewBusinessRaftClientUsesConfiguredWriteTimeout(t *testing.T) {
	const writeTimeout = 7 * time.Second
	client := NewBusinessRaftClient(raft2.ClusterIDKeyStore, nil, config.Cluster{
		WriteTimeout: writeTimeout,
	})

	if got := client.WriteTimeout(); got != writeTimeout {
		t.Fatalf("write timeout = %v, want %v", got, writeTimeout)
	}
	if got := client.ReadTimeout(); got != 5*time.Second {
		t.Fatalf("read timeout = %v, want %v", got, 5*time.Second)
	}
}

// TestClusterDescriptorByIDReturnsErrorForUnknownID 验证未知 cluster id 会返回错误。
func TestClusterDescriptorByIDReturnsErrorForUnknownID(t *testing.T) {
	_, err := ClusterDescriptorByID(1)
	if err == nil {
		t.Fatal("expected error for unknown cluster id")
	}
	if !strings.Contains(err.Error(), "cluster descriptor not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
