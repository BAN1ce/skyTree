package clusterruntime

import (
	"fmt"

	"github.com/BAN1ce/skyTree/config"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/cluster/raftcfg"
)

// NewRaftCluster 根据集群配置创建 raft cluster 实例。
func NewRaftCluster(clusterConfig config.Cluster) *raft2.Cluster {
	return raft2.NewCluster(
		raft2.WithInitialMembers(raftcfg.InitialMembers(clusterConfig)),
		raft2.WithJoin(clusterConfig.Join),
		raft2.WithNodeHostConfig(raftcfg.NodeHostConfig(clusterConfig)),
	)
}

// NewBusinessRaftClient 为指定 raft group 创建业务读写客户端。
func NewBusinessRaftClient(
	clusterID uint64,
	cluster *raft2.Cluster,
	clusterConfig config.Cluster,
) *raft2.Client {
	return raft2.NewClient(clusterID, cluster, raft2.WithTimeout(0, clusterConfig.WriteTimeout))
}

// ClusterDescriptorByID 根据 cluster id 查找系统 raft group 描述信息。
func ClusterDescriptorByID(clusterID uint64) (raft2.ClusterDescriptor, error) {
	descriptor, ok := raft2.ClusterDescriptorByID(clusterID)
	if !ok {
		return raft2.ClusterDescriptor{}, fmt.Errorf("cluster descriptor not found: cluster_id=%d", clusterID)
	}
	return descriptor, nil
}
