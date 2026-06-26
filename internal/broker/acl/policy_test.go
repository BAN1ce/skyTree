package acl

import (
	"context"
	"testing"
)

func TestStaticEvaluator_AllowPublish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ev := NewStaticEvaluator(Ruleset{
		DefaultDeny: true,
		Rules: []Rule{
			{
				Identity: Identity{Username: "u1"},
				Allow:    RuleEntry{Pub: []string{"a/+"}},
				Deny:     RuleEntry{Pub: []string{"a/b"}},
			},
		},
	})

	if ok, err := ev.AllowPublish(ctx, "u1", "", ""); err == nil || ok {
		t.Fatalf("empty topic should error and deny; ok=%v err=%v", ok, err)
	}

	// Deny has higher priority than allow.
	if ok, err := ev.AllowPublish(ctx, "u1", "", "a/b"); err != nil || ok {
		t.Fatalf("deny should win; ok=%v err=%v", ok, err)
	}

	if ok, err := ev.AllowPublish(ctx, "u1", "", "a/c"); err != nil || !ok {
		t.Fatalf("allow should pass; ok=%v err=%v", ok, err)
	}

	// No matching identity -> default deny.
	if ok, err := ev.AllowPublish(ctx, "u2", "", "a/c"); err != nil || ok {
		t.Fatalf("default deny expected; ok=%v err=%v", ok, err)
	}
}

func TestStaticEvaluator_AllowSubscribe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ev := NewStaticEvaluator(Ruleset{
		DefaultDeny: true,
		Rules: []Rule{
			{
				Identity: Identity{ClientID: "c1"},
				Allow:    RuleEntry{Sub: []string{"a/#"}},
				Deny:     RuleEntry{Sub: []string{"a/+/c"}},
			},
		},
	})

	if ok, err := ev.AllowSubscribe(ctx, "", "c1", ""); err == nil || ok {
		t.Fatalf("empty topic filter should error and deny; ok=%v err=%v", ok, err)
	}

	// Deny is checked by FilterSuperset(ruleFilter, reqFilter).
	// Here, rule "a/+/c" is a superset of req "a/+/c", so it denies.
	if ok, err := ev.AllowSubscribe(ctx, "", "c1", "a/+/c"); err != nil || ok {
		t.Fatalf("deny should win; ok=%v err=%v", ok, err)
	}

	// Allow when allow is a superset and no deny matches.
	if ok, err := ev.AllowSubscribe(ctx, "", "c1", "a/+/d"); err != nil || !ok {
		t.Fatalf("allow should pass; ok=%v err=%v", ok, err)
	}

	// No matching identity -> default deny.
	if ok, err := ev.AllowSubscribe(ctx, "", "c2", "a/+/d"); err != nil || ok {
		t.Fatalf("default deny expected; ok=%v err=%v", ok, err)
	}
}

func TestStaticEvaluator_DefaultAllowWhenNotDefaultDeny(t *testing.T) {
	t.Parallel()

	ev := NewStaticEvaluator(Ruleset{
		DefaultDeny: false,
		Rules:       nil,
	})

	ok, err := ev.AllowPublish(context.Background(), "u", "c", "any/topic")
	if err != nil || !ok {
		t.Fatalf("default allow expected; ok=%v err=%v", ok, err)
	}
}

func TestIdentityMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		id       Identity
		username string
		clientID string
		want     bool
	}{
		{name: "empty_identity_never_matches", id: Identity{}, username: "u", clientID: "c", want: false},
		{name: "username_exact", id: Identity{Username: "u"}, username: "u", clientID: "c", want: true},
		{name: "username_mismatch", id: Identity{Username: "u"}, username: "x", clientID: "c", want: false},
		{name: "client_id_exact", id: Identity{ClientID: "c"}, username: "u", clientID: "c", want: true},
		{name: "client_id_mismatch", id: Identity{ClientID: "c"}, username: "u", clientID: "x", want: false},
		{name: "both_match", id: Identity{Username: "u", ClientID: "c"}, username: "u", clientID: "c", want: true},
		{name: "both_one_mismatch", id: Identity{Username: "u", ClientID: "c"}, username: "u", clientID: "x", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := identityMatch(tc.id, tc.username, tc.clientID); got != tc.want {
				t.Fatalf("identityMatch(%+v,%q,%q)=%v want=%v", tc.id, tc.username, tc.clientID, got, tc.want)
			}
		})
	}
}
