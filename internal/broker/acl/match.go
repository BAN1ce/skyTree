package acl

import topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"

// MatchTopic returns true when topic matches the given MQTT topic filter.
// - filter may contain '+' and '#'
// - topic must be a concrete topic (no wildcards)
func MatchTopic(filter, topic string) bool {
	if filter == "" || topic == "" {
		return false
	}
	if topicutil.HasWildcard(topic) {
		return false
	}
	fLevels := topicutil.SplitTopicLevels(filter)
	tLevels := topicutil.SplitTopicLevels(topic)
	return matchLevels(fLevels, tLevels)
}

// FilterSuperset returns true when ruleFilter covers all topics matched by reqFilter.
// This is used for SUBSCRIBE authorization where reqFilter itself may include wildcards.
//
// Notes:
// - This is intentionally conservative.
// - It assumes MQTT topics are non-empty (so "+/#" covers all valid topics).
func FilterSuperset(ruleFilter, reqFilter string) bool {
	if ruleFilter == "" || reqFilter == "" {
		return false
	}
	if ruleFilter == "#" {
		return true
	}
	if reqFilter == "#" {
		return ruleFilter == "#" || ruleFilter == "+/#"
	}

	rLevels := topicutil.SplitTopicLevels(ruleFilter)
	qLevels := topicutil.SplitTopicLevels(reqFilter)
	return supersetLevels(rLevels, qLevels)
}

func matchLevels(filterLevels, topicLevels []string) bool {
	for i := 0; i < len(filterLevels); i++ {
		f := filterLevels[i]
		if f == "#" {
			return true
		}
		if i >= len(topicLevels) {
			return false
		}
		if f == "+" {
			continue
		}
		if f != topicLevels[i] {
			return false
		}
	}
	return len(topicLevels) == len(filterLevels)
}

func supersetLevels(ruleLevels, reqLevels []string) bool {
	i, j := 0, 0
	for {
		if j >= len(ruleLevels) {
			return i >= len(reqLevels)
		}
		r := ruleLevels[j]
		if r == "#" {
			return true
		}
		if i >= len(reqLevels) {
			return false
		}

		q := reqLevels[i]
		switch q {
		case "#":
			// reqFilter matches arbitrary suffix; only a rule '#' can safely cover it (handled above).
			return false
		case "+":
			// reqFilter matches any single level; rule must also match any single level here.
			if r != "+" {
				return false
			}
		default:
			// reqFilter matches a concrete string (literal level).
			if r != "+" && r != q {
				return false
			}
		}

		i++
		j++
	}
}
