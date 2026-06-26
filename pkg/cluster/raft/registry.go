package raft

import "fmt"

const (
	ClusterNameSubCenter       = "sub_center"
	ClusterNameKeyStore        = "key_store"
	ClusterNameSessionCenter   = "session_center"
	ClusterNameWillDelayCenter = "will_delay_center"
)

type ClusterKind string

const (
	ClusterKindSystem ClusterKind = "system"
)

type HealthCheckMode string

const (
	HealthCheckEveryInterval HealthCheckMode = "every_interval"
)

type ClusterDescriptor struct {
	ClusterID         uint64
	Name              string
	Kind              ClusterKind
	StateMachineOwner string
	HealthCheck       HealthCheckMode
	ShardCount        int
	ShardID           int
}

var systemClusterRegistry = []ClusterDescriptor{
	{
		ClusterID:         ClusterIDSubCenter,
		Name:              ClusterNameSubCenter,
		Kind:              ClusterKindSystem,
		StateMachineOwner: ClusterNameSubCenter,
		HealthCheck:       HealthCheckEveryInterval,
		ShardCount:        1,
		ShardID:           -1,
	},
	{
		ClusterID:         ClusterIDKeyStore,
		Name:              ClusterNameKeyStore,
		Kind:              ClusterKindSystem,
		StateMachineOwner: ClusterNameKeyStore,
		HealthCheck:       HealthCheckEveryInterval,
		ShardCount:        1,
		ShardID:           -1,
	},
	{
		ClusterID:         ClusterIDSessionCenter,
		Name:              ClusterNameSessionCenter,
		Kind:              ClusterKindSystem,
		StateMachineOwner: ClusterNameSessionCenter,
		HealthCheck:       HealthCheckEveryInterval,
		ShardCount:        1,
		ShardID:           -1,
	},
	{
		ClusterID:         ClusterIDWillDelayCenter,
		Name:              ClusterNameWillDelayCenter,
		Kind:              ClusterKindSystem,
		StateMachineOwner: ClusterNameWillDelayCenter,
		HealthCheck:       HealthCheckEveryInterval,
		ShardCount:        1,
		ShardID:           -1,
	},
}

func SystemClusterDescriptors() []ClusterDescriptor {
	out := make([]ClusterDescriptor, len(systemClusterRegistry))
	copy(out, systemClusterRegistry)
	return out
}

func MustClusterDescriptor(clusterID uint64) ClusterDescriptor {
	descriptor, ok := ClusterDescriptorByID(clusterID)
	if !ok {
		panic(fmt.Sprintf("unknown raft cluster id %d", clusterID))
	}
	return descriptor
}

func ClusterDescriptorByID(clusterID uint64) (ClusterDescriptor, bool) {
	for _, descriptor := range systemClusterRegistry {
		if descriptor.ClusterID == clusterID {
			return descriptor, true
		}
	}
	return ClusterDescriptor{}, false
}

func SystemClusterIDs() []uint64 {
	descriptors := SystemClusterDescriptors()
	ids := make([]uint64, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ClusterID)
	}
	return ids
}

func ClusterName(clusterID uint64) string {
	if descriptor, ok := ClusterDescriptorByID(clusterID); ok {
		return descriptor.Name
	}
	return fmt.Sprintf("cluster_%d", clusterID)
}
