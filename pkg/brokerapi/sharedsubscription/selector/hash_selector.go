package selector

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// HashSelector selects a client based on hash of message content
// This ensures the same message is routed to the same client
type HashSelector struct{}

// NewHashSelector creates a new hash selector
func NewHashSelector() *HashSelector {
	return &HashSelector{}
}

// Select selects a client based on hash of message content
func (s *HashSelector) Select(shareGroup string, members []*sharedsubscription.ShareGroupMember, publish *packets.Publish) string {
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

	// Compute hash from message content
	hash := s.computeHash(shareGroup, publish)

	// Select client based on hash
	index := int(hash % uint64(len(validMembers)))
	return validMembers[index].ClientID
}

// computeHash computes a hash from shareGroup and message content
func (s *HashSelector) computeHash(shareGroup string, publish *packets.Publish) uint64 {
	h := sha256.New()
	h.Write([]byte(shareGroup))
	if publish != nil {
		if publish.Topic != "" {
			h.Write([]byte(publish.Topic))
		}
		if len(publish.Payload) > 0 {
			h.Write(publish.Payload)
		}
		// Include message ID if available for better distribution
		if publish.PacketID != 0 {
			var buf [8]byte
			binary.BigEndian.PutUint64(buf[:], uint64(publish.PacketID))
			h.Write(buf[:])
		}
	}
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}
