package acl

import "testing"

func TestMatchTopic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		filter string
		topic  string
		want   bool
	}{
		{name: "empty_filter", filter: "", topic: "a/b", want: false},
		{name: "empty_topic", filter: "a/b", topic: "", want: false},
		{name: "topic_has_wildcard_plus", filter: "a/+", topic: "a/+", want: false},
		{name: "topic_has_wildcard_hash", filter: "a/#", topic: "a/#", want: false},

		{name: "exact_match", filter: "a/b", topic: "a/b", want: true},
		{name: "exact_mismatch_extra_level", filter: "a/b", topic: "a/b/c", want: false},
		{name: "plus_single_level", filter: "a/+", topic: "a/b", want: true},
		{name: "plus_missing_level", filter: "a/+", topic: "a", want: false},
		{name: "hash_matches_suffix", filter: "a/#", topic: "a/b/c", want: true},
		{name: "hash_matches_empty_suffix", filter: "a/b/#", topic: "a/b", want: true},
		{name: "root_hash", filter: "#", topic: "a/b", want: true},
		{name: "single_level_plus", filter: "+", topic: "a", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchTopic(tc.filter, tc.topic); got != tc.want {
				t.Fatalf("MatchTopic(%q,%q)=%v want=%v", tc.filter, tc.topic, got, tc.want)
			}
		})
	}
}

func TestFilterSuperset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		ruleFilter string
		reqFilter  string
		want       bool
	}{
		{name: "empty_rule", ruleFilter: "", reqFilter: "a/#", want: false},
		{name: "empty_req", ruleFilter: "#", reqFilter: "", want: false},

		{name: "rule_hash_is_superset", ruleFilter: "#", reqFilter: "a/+", want: true},
		{name: "req_hash_only_covered_by_hash_or_plus_hash", ruleFilter: "+/#", reqFilter: "#", want: true},
		{name: "req_hash_not_covered_by_other", ruleFilter: "a/#", reqFilter: "#", want: false},

		{name: "rule_literal_covers_exact_literal", ruleFilter: "a/b", reqFilter: "a/b", want: true},
		{name: "rule_plus_covers_literal", ruleFilter: "a/+", reqFilter: "a/b", want: true},
		{name: "rule_literal_not_cover_req_plus", ruleFilter: "a/b", reqFilter: "a/+", want: false},
		{name: "rule_hash_covers_req_plus_suffix", ruleFilter: "a/#", reqFilter: "a/+/c", want: true},

		{name: "req_hash_suffix_only_covered_by_root_hash", ruleFilter: "#", reqFilter: "a/#", want: true},
		{name: "req_hash_suffix_covered_by_same_prefix_hash", ruleFilter: "a/#", reqFilter: "a/#", want: true},
		{name: "req_hash_in_middle_still_covered_when_rule_has_hash", ruleFilter: "a/#", reqFilter: "a/#/b", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FilterSuperset(tc.ruleFilter, tc.reqFilter); got != tc.want {
				t.Fatalf("FilterSuperset(%q,%q)=%v want=%v", tc.ruleFilter, tc.reqFilter, got, tc.want)
			}
		})
	}
}
