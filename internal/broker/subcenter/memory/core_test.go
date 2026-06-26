package memory

import (
	"testing"

	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

func TestMatchTopicClient_HashMatchesZeroLevels(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a/#"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	got := s.matchTopicClient("a")
	opt, ok := got["c1"]
	if !ok || opt == nil {
		t.Fatalf("expected client c1 to match, got: %v", got)
	}
	if opt.GetQoS() != 1 {
		t.Fatalf("expected QoS=1, got %d", opt.GetQoS())
	}
}

func TestMatchTopicClient_RootHashMatchesLeadingSlashTopic(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "#"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	got := s.matchTopicClient("/a/b")
	opt, ok := got["c1"]
	if !ok || opt == nil {
		t.Fatalf("expected client c1 to match, got: %v", got)
	}
	if opt.GetQoS() != 1 {
		t.Fatalf("expected QoS=1, got %d", opt.GetQoS())
	}
}

func TestMatchTopicClient_MergeByMaxQoS(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a/b"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if err := s.createSub("c1", &proto.SubOption{QoS: 0, Topic: "a/+"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	got := s.matchTopicClient("a/b")
	opt, ok := got["c1"]
	if !ok || opt == nil {
		t.Fatalf("expected client c1 to match, got: %v", got)
	}
	if opt.GetQoS() != 1 {
		t.Fatalf("expected QoS=1, got %d (topic=%q)", opt.GetQoS(), opt.GetTopic())
	}
	if opt.GetTopic() != "a/b" {
		t.Fatalf("expected chosen topic to be %q, got %q", "a/b", opt.GetTopic())
	}
}

func TestCreateSub_ReSubscribeReplacesExistingOptions(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{
		QoS:                    0,
		Topic:                  "a/b",
		RetainAsPublished:      false,
		NoLocal:                false,
		RetainHandling:         0,
		SubscriptionIdentifier: 1,
	}); err != nil {
		t.Fatalf("first createSub error: %v", err)
	}
	if err := s.createSub("c1", &proto.SubOption{
		QoS:                    2,
		Topic:                  "a/b",
		RetainAsPublished:      true,
		NoLocal:                true,
		RetainHandling:         2,
		SubscriptionIdentifier: 9,
	}); err != nil {
		t.Fatalf("second createSub error: %v", err)
	}

	got := s.matchTopicClient("a/b")["c1"]
	if got == nil {
		t.Fatalf("expected c1 subscription")
	}
	if got.GetQoS() != 2 {
		t.Fatalf("expected updated QoS=2, got %d", got.GetQoS())
	}
	if !got.GetRetainAsPublished() || !got.GetNoLocal() {
		t.Fatalf("expected updated RAP/NL true, got RAP=%v NL=%v", got.GetRetainAsPublished(), got.GetNoLocal())
	}
	if got.GetRetainHandling() != 2 || got.GetSubscriptionIdentifier() != 9 {
		t.Fatalf("expected updated RH=2 SubID=9, got RH=%d SubID=%d", got.GetRetainHandling(), got.GetSubscriptionIdentifier())
	}
}

func TestMatchTopicClient_PlusSingleLevel(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a/+"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if _, ok := s.matchTopicClient("a/b")["c1"]; !ok {
		t.Fatalf("expected match for a/b")
	}
	if _, ok := s.matchTopicClient("a/b/c")["c1"]; ok {
		t.Fatalf("did not expect match for a/b/c")
	}
}

func TestMatchTopicClient_PlusMatchesEmptyLevel(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "+/b"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if _, ok := s.matchTopicClient("/b")["c1"]; !ok {
		t.Fatalf("expected match for /b")
	}
}

func TestMatchTopicClient_HashMatchesMultipleLevelsAndEmptyLevels(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a/#"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if _, ok := s.matchTopicClient("a/b/c")["c1"]; !ok {
		t.Fatalf("expected match for a/b/c")
	}
	if _, ok := s.matchTopicClient("a/")["c1"]; !ok {
		t.Fatalf("expected match for a/")
	}
	if _, ok := s.matchTopicClient("a//b")["c1"]; !ok {
		t.Fatalf("expected match for a//b")
	}
}

func TestMatchTopicClient_MultiClient(t *testing.T) {
	s := NewSubCore()
	_ = s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a/#"})
	_ = s.createSub("c2", &proto.SubOption{QoS: 0, Topic: "a/+"})
	got := s.matchTopicClient("a/b")
	if got["c1"] == nil || got["c2"] == nil {
		t.Fatalf("expected both clients to match, got: %v", got)
	}
}

func TestCreateSub_InvalidFiltersRejected(t *testing.T) {
	s := NewSubCore()
	cases := []string{
		"a/#/b",
		"#/a/b",
		"a+",
		"a/+#",
	}
	for _, tc := range cases {
		if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: tc}); err == nil {
			t.Fatalf("expected error for invalid filter %q", tc)
		}
	}
}

func TestMatchTopicClient_PublishTopicWithWildcardRejected(t *testing.T) {
	s := NewSubCore()
	_ = s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "#"})
	if got := s.matchTopicClient("a/+"); len(got) != 0 {
		t.Fatalf("expected empty result for wildcard publish topic, got: %v", got)
	}
}

func TestMatchTopicClient_SysTopicRootWildcardDoesNotMatch(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("cHash", &proto.SubOption{QoS: 1, Topic: "#"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if err := s.createSub("cPlus", &proto.SubOption{QoS: 1, Topic: "+"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if err := s.createSub("cSys", &proto.SubOption{QoS: 1, Topic: "$SYS/#"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}

	got := s.matchTopicClient("$SYS/a")
	if got["cHash"] != nil || got["cPlus"] != nil {
		t.Fatalf("did not expect root wildcards to match $SYS, got: %v", got)
	}
	if got["cSys"] == nil {
		t.Fatalf("expected $SYS/# to match $SYS/a, got: %v", got)
	}
}

func TestMatchTopicClient_DollarTopicRootWildcardDoesNotMatch(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("cHash", &proto.SubOption{QoS: 1, Topic: "#"}); err != nil {
		t.Fatalf("createSub hash: %v", err)
	}
	if err := s.createSub("cPlus", &proto.SubOption{QoS: 1, Topic: "+"}); err != nil {
		t.Fatalf("createSub plus: %v", err)
	}
	if err := s.createSub("cDollar", &proto.SubOption{QoS: 1, Topic: "$foo/#"}); err != nil {
		t.Fatalf("createSub dollar: %v", err)
	}

	got := s.matchTopicClient("$foo/a")
	if got["cHash"] != nil || got["cPlus"] != nil {
		t.Fatalf("did not expect root wildcards to match $foo, got: %v", got)
	}
	if got["cDollar"] == nil {
		t.Fatalf("expected $foo/# to match $foo/a, got: %v", got)
	}
}

func TestMatchTopicClientV2_SharedAndNormalBothMatch(t *testing.T) {
	s := NewSubCore()
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "a"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}
	if err := s.createSub("c1", &proto.SubOption{QoS: 1, Topic: "$share/g/a"}); err != nil {
		t.Fatalf("createSub error: %v", err)
	}

	got := s.matchTopicClientV2("a")
	subs := got["c1"]
	if len(subs) != 2 {
		t.Fatalf("expected 2 matched subscriptions, got %v", subs)
	}
	filters := []string{subs[0].GetTopicFilter(), subs[1].GetTopicFilter()}
	// Order is not guaranteed.
	if !((filters[0] == "a" && filters[1] == "$share/g/a") || (filters[0] == "$share/g/a" && filters[1] == "a")) {
		t.Fatalf("unexpected matched filters: %v", filters)
	}
}
