package sharedsubscription

import (
	"fmt"
	"strings"

	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
)

const (
	// SharedSubscriptionPrefix is the prefix for shared subscription topic filters
	SharedSubscriptionPrefix = "$share/"
)

// ParseSharedSubscription parses a shared subscription topic filter
// Format: $share/{ShareName}/{TopicFilter}
// Returns: (shareGroup, actualTopicFilter, error)
func ParseSharedSubscription(topicFilter string) (shareGroup string, actualTopicFilter string, err error) {
	if !IsSharedSubscription(topicFilter) {
		return "", "", fmt.Errorf("not a shared subscription: %s", topicFilter)
	}

	// Remove the $share/ prefix
	withoutPrefix := topicFilter[len(SharedSubscriptionPrefix):]
	if withoutPrefix == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: missing share name")
	}

	// Split by '/' to get share name and topic filter
	levels := topicutil.SplitTopicLevels(withoutPrefix)
	if len(levels) < 2 {
		return "", "", fmt.Errorf("invalid shared subscription format: missing topic filter")
	}

	shareGroup = levels[0]
	if shareGroup == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: empty share name")
	}
	if strings.ContainsAny(shareGroup, "+#") {
		return "", "", fmt.Errorf("invalid shared subscription format: share name contains wildcard")
	}

	// Reconstruct the actual topic filter from remaining levels
	actualTopicFilter = strings.Join(levels[1:], "/")
	if actualTopicFilter == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: empty topic filter")
	}

	return shareGroup, actualTopicFilter, nil
}

// IsSharedSubscription checks if a topic filter is a shared subscription
func IsSharedSubscription(topicFilter string) bool {
	return strings.HasPrefix(topicFilter, SharedSubscriptionPrefix)
}

// BuildSharedSubscriptionTopicFilter builds a shared subscription topic filter
// Format: $share/{ShareName}/{TopicFilter}
func BuildSharedSubscriptionTopicFilter(shareGroup string, topicFilter string) string {
	return SharedSubscriptionPrefix + shareGroup + "/" + topicFilter
}
