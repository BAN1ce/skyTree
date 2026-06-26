package sharedsubscription

import "testing"

func TestParseSharedSubscriptionRejectsWildcardShareName(t *testing.T) {
	tests := []struct {
		name  string
		topic string
	}{
		{name: "plus", topic: "$share/+/a/b"},
		{name: "hash", topic: "$share/#/a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ParseSharedSubscription(tt.topic); err == nil {
				t.Fatalf("expected wildcard share name %q to be rejected", tt.topic)
			}
		})
	}
}
