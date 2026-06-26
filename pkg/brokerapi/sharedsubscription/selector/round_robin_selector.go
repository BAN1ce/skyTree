package selector

import (
	"sync"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// RoundRobinSelector selects clients in a round-robin fashion
type RoundRobinSelector struct {
	mu       sync.Mutex
	counters map[string]int // shareGroup -> counter
}

// NewRoundRobinSelector creates a new round-robin selector
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{
		counters: make(map[string]int),
	}
}

// Select selects a client in round-robin fashion
func (s *RoundRobinSelector) Select(shareGroup string, members []*sharedsubscription.ShareGroupMember, publish *packets.Publish) string {
	if len(members) == 0 {
		return ""
	}

	// Members are already filtered to be online by the consumer
	// Filter out nil members
	validMembers := make([]*sharedsubscription.ShareGroupMember, 0, len(members))
	for _, member := range members {
		if member != nil && member.ClientID != "" {
			validMembers = append(validMembers, member)
		}
	}

	if len(validMembers) == 0 {
		return ""
	}

	s.mu.Lock()
	counter := s.counters[shareGroup]
	s.counters[shareGroup] = (counter + 1) % len(validMembers)
	index := counter
	s.mu.Unlock()

	return validMembers[index].ClientID
}
