package acl

import (
	"context"
	"fmt"
)

// Evaluator evaluates ACL decisions for publish and subscribe.
type Evaluator interface {
	AllowPublish(ctx context.Context, username, clientID, topic string) (bool, error)
	AllowSubscribe(ctx context.Context, username, clientID, topicFilter string) (bool, error)
}

// StaticEvaluator evaluates decisions against an in-memory ruleset.
type StaticEvaluator struct {
	ruleset Ruleset
}

func NewStaticEvaluator(r Ruleset) *StaticEvaluator {
	return &StaticEvaluator{ruleset: r}
}

func (e *StaticEvaluator) AllowPublish(_ context.Context, username, clientID, topic string) (bool, error) {
	if topic == "" {
		return false, fmt.Errorf("empty topic")
	}
	return e.eval(username, clientID, ActionPublish, topic), nil
}

func (e *StaticEvaluator) AllowSubscribe(_ context.Context, username, clientID, topicFilter string) (bool, error) {
	if topicFilter == "" {
		return false, fmt.Errorf("empty topic filter")
	}
	return e.eval(username, clientID, ActionSubscribe, topicFilter), nil
}

func (e *StaticEvaluator) eval(username, clientID string, action Action, subject string) bool {
	// Deny first, then allow; else default decision.
	for _, r := range e.ruleset.Rules {
		if !identityMatch(r.Identity, username, clientID) {
			continue
		}
		if action == ActionPublish {
			if anyMatchTopic(r.Deny.Pub, subject) {
				return false
			}
		} else {
			if anySuperset(r.Deny.Sub, subject) {
				return false
			}
		}
	}

	for _, r := range e.ruleset.Rules {
		if !identityMatch(r.Identity, username, clientID) {
			continue
		}
		if action == ActionPublish {
			if anyMatchTopic(r.Allow.Pub, subject) {
				return true
			}
		} else {
			if anySuperset(r.Allow.Sub, subject) {
				return true
			}
		}
	}

	return !e.ruleset.DefaultDeny
}

func identityMatch(id Identity, username, clientID string) bool {
	if id.Username != "" && id.Username != username {
		return false
	}
	if id.ClientID != "" && id.ClientID != clientID {
		return false
	}
	return id.Username != "" || id.ClientID != ""
}

func anyMatchTopic(filters []string, topic string) bool {
	for _, f := range filters {
		if MatchTopic(f, topic) {
			return true
		}
	}
	return false
}

func anySuperset(ruleFilters []string, reqFilter string) bool {
	for _, rf := range ruleFilters {
		if FilterSuperset(rf, reqFilter) {
			return true
		}
	}
	return false
}
