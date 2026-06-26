package topic

import "strings"

type topicFilterValidationError string

func (e topicFilterValidationError) Error() string {
	return string(e)
}

const (
	errTopicFilterEmpty              topicFilterValidationError = "topic filter must be non-empty"
	errSharedSubscriptionFormat      topicFilterValidationError = "shared subscription must be $share/{group}/{filter}"
	errSharedSubscriptionGroupSyntax topicFilterValidationError = "shared subscription share name must not contain wildcards"
	errMultiWildcardSyntax           topicFilterValidationError = "multi-level wildcard must occupy the final segment"
	errSingleWildcardSyntax          topicFilterValidationError = "single-level wildcard must occupy an entire segment"
)

// ValidateTopicFilterSyntax validates MQTT topic-filter wildcard syntax.
//
// It accepts both ordinary filters and shared subscription filters.
func ValidateTopicFilterSyntax(filter string) error {
	if filter == "" {
		return errTopicFilterEmpty
	}
	parsedFilter := filter
	if strings.HasPrefix(parsedFilter, "$share/") {
		rest := strings.TrimPrefix(parsedFilter, "$share/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return errSharedSubscriptionFormat
		}
		if strings.ContainsAny(parts[0], "+#") {
			return errSharedSubscriptionGroupSyntax
		}
		parsedFilter = parts[1]
	}

	levels := SplitTopicLevels(parsedFilter)
	for idx, level := range levels {
		if level == "#" {
			if idx != len(levels)-1 {
				return errMultiWildcardSyntax
			}
			continue
		}
		if strings.Contains(level, "#") {
			return errMultiWildcardSyntax
		}
		if strings.Contains(level, "+") && level != "+" {
			return errSingleWildcardSyntax
		}
	}
	return nil
}
