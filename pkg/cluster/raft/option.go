package raft

import "github.com/lni/dragonboat/v3/config"

type Option func(*Cluster)

func WithInitialMembers(members map[uint64]string) Option {
	return func(cluster *Cluster) {
		cluster.initialMembers = members
	}
}

func WithJoin(join bool) Option {
	return func(cluster *Cluster) {
		cluster.join = join
	}
}

func WithNodeHostConfig(nodeConfig config.NodeHostConfig) Option {
	return func(cluster *Cluster) {
		cluster.nodeConfig = nodeConfig
	}
}
