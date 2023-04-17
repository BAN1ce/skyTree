package util

import "strings"

func SplitTopic(topic string) []string {
	tmp := strings.Split(strings.Trim(topic, "/"), "/")
	for _, v := range tmp {
		if v == "" {
			result := make([]string, 0)
			for _, v := range tmp {
				if v != "" {
					result = append(result, v)
				}
			}
			return result
		}
	}
	return tmp
}

func HasWildcard(topic string) bool {
	return strings.Contains(topic, "+") || strings.Contains(topic, "#")
}

func subQosMoreThan0(topics map[string]int32) bool {
	for _, v := range topics {
		if v > 0 {
			return true
		}
	}
	return false
}
