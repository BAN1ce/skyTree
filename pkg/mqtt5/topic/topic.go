package topic

import (
	"github.com/google/uuid"
	"math/rand"
	"strings"
	"time"
)

// SplitTopicLevels splits a MQTT topic/topic-filter into levels by '/' and preserves empty levels.
// - Leading '/' produces a leading empty level (e.g. "/a" -> ["", "a"])
// - Trailing '/' produces a trailing empty level (e.g. "a/" -> ["a", ""])
// - Consecutive '/' produces empty levels (e.g. "a//b" -> ["a", "", "b"])
// Note: For topic == "", it returns nil.
func SplitTopicLevels(topic string) []string {
	if topic == "" {
		return nil
	}
	return strings.Split(topic, "/")
}

func SplitTopic(topic string) []string {
	if len(topic) == 0 {
		return nil
	}
	var result = make([]string, 0, 30)

	if topic[0] == '/' {
		result = append(result, "/")

	}
	//if !strings.Contains(topic, "/") {
	//	result = append(result, "/")
	//}

	trimmed := strings.Trim(topic, "/")
	if trimmed == "" {
		// Topic is "/" or contains only '/'.
		tmp := result
		for _, v := range tmp {
			if v != "/" {
				return tmp
			}
		}
		return nil
	}

	tmp := strings.Split(trimmed, "/")

	for _, v := range tmp {
		result = append(result, v)
	}

	for _, v := range result {
		if v != "/" {
			return result
		}
	}

	return nil

}

// nolint
func HasWildcard(topic string) bool {
	return strings.Contains(topic, "+") || strings.Contains(topic, "#")
}

// MatchTopicFilter reports whether a concrete MQTT topic name matches a topic filter.
// It preserves MQTT's rule that a leading wildcard does not match topics starting with '$'.
func MatchTopicFilter(filter, topic string) bool {
	if filter == "" || topic == "" {
		return false
	}
	filterLevels := SplitTopicLevels(filter)
	topicLevels := SplitTopicLevels(topic)
	if len(filterLevels) == 0 || len(topicLevels) == 0 {
		return false
	}
	if strings.HasPrefix(topicLevels[0], "$") && !strings.HasPrefix(filterLevels[0], "$") {
		return false
	}

	for idx, filterLevel := range filterLevels {
		switch filterLevel {
		case "#":
			return idx == len(filterLevels)-1
		case "+":
			if idx >= len(topicLevels) {
				return false
			}
			continue
		default:
			if idx >= len(topicLevels) || filterLevel != topicLevels[idx] {
				return false
			}
		}
	}
	return len(filterLevels) == len(topicLevels)
}

// nolint
func subQosMoreThan0(topics map[string]int32) bool {
	for _, v := range topics {
		if v > 0 {
			return true
		}
	}
	return false
}

func ParseShareTopic(shareTopic string) (shareGroup, subTopic string) {
	if !IsShareTopic(shareTopic) {
		return "", shareTopic
	}
	shareNameSubTopic := strings.TrimPrefix(shareTopic, "$share/")

	index := strings.Index(shareNameSubTopic, "/")
	if index == -1 {
		return "", ""
	}
	return shareNameSubTopic[:index], shareNameSubTopic[index+1:]
}

func IsShareTopic(shareTopic string) bool {
	return strings.HasPrefix(shareTopic, "$share/")
}

func RandomWildcardTopic() string {
	var (
		result       = "/"
		randomLength = rand.Intn(20)
	)

	for i := 0; i < randomLength; i++ {
		switch time.Now().Nanosecond() % 3 {
		case 0:
			result += "/+"
		case 1:
			result += "/#"
			return result
		case 2:
			result += "/" + uuid.NewString()

		}

	}
	return result
}
