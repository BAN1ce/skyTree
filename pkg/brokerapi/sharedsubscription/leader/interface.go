package leader

import (
	"context"
	"time"
)

// LeaderElection manages leader election for shared subscription groups
type LeaderElection interface {
	// TryAcquireLeadership tries to acquire leadership for a resource
	// Returns true if successfully acquired, false if another node is already the leader
	TryAcquireLeadership(ctx context.Context, resourceID string, nodeID uint64, ttl time.Duration) (bool, error)

	// RenewLeadership renews the leadership lease
	RenewLeadership(ctx context.Context, resourceID string, nodeID uint64, ttl time.Duration) error

	// ReleaseLeadership releases the leadership
	ReleaseLeadership(ctx context.Context, resourceID string, nodeID uint64) error

	// WatchLeadership watches for leadership changes and calls callback when leadership changes
	WatchLeadership(ctx context.Context, resourceID string, callback func(isLeader bool)) error
}
