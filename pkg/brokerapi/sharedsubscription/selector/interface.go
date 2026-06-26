package selector

import (
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ShareGroupSelector selects a client from a list of online clients for a shared subscription group
type ShareGroupSelector interface {
	// Select selects a client from the list of online members
	// Returns empty string if no client is available
	Select(shareGroup string, members []*sharedsubscription.ShareGroupMember, publish *packets.Publish) string
}
