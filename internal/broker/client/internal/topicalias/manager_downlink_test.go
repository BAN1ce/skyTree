package topicalias

import (
	"sync"
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestApplyTopicAliasToClient_Disabled_StripsExistingAlias(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(0)
	m.ResetDownlink()

	alias := uint16(7)
	p := &packets.Publish{
		Topic:      "a/b",
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}
	m.ApplyDownlink(p)

	if p.Topic != "a/b" {
		t.Fatalf("expected topic kept, got %q", p.Topic)
	}
	if p.Properties != nil && p.Properties.TopicAlias != nil {
		t.Fatalf("expected alias stripped when disabled, got %v", *p.Properties.TopicAlias)
	}
}

func TestApplyTopicAliasToClient_FirstSend_SetsAliasAndKeepsTopic(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(10)
	m.ResetDownlink()

	p := &packets.Publish{
		Topic:      "a/b",
		Properties: &packets.PublishProperties{},
	}
	m.ApplyDownlink(p)

	if p.Topic != "a/b" {
		t.Fatalf("expected topic kept on first send, got %q", p.Topic)
	}
	if p.Properties == nil || p.Properties.TopicAlias == nil || *p.Properties.TopicAlias != 1 {
		t.Fatalf("expected alias=1 on first send, got %+v", p.Properties)
	}
}

func TestApplyTopicAliasToClient_SecondSend_OmitsTopic(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(10)
	m.ResetDownlink()

	p1 := &packets.Publish{Topic: "a/b", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p1)

	p2 := &packets.Publish{Topic: "a/b", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p2)

	if p2.Topic != "" {
		t.Fatalf("expected topic omitted on repeat, got %q", p2.Topic)
	}
	if p2.Properties == nil || p2.Properties.TopicAlias == nil || *p2.Properties.TopicAlias != 1 {
		t.Fatalf("expected alias=1 on repeat, got %+v", p2.Properties)
	}
}

func TestApplyTopicAliasToClient_Exhausted_FallsBackToFullTopic(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(1)
	m.ResetDownlink()

	p1 := &packets.Publish{Topic: "a/b", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p1)

	p2 := &packets.Publish{Topic: "c/d", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p2)

	if p2.Topic != "c/d" {
		t.Fatalf("expected full topic when exhausted, got %q", p2.Topic)
	}
	if p2.Properties != nil && p2.Properties.TopicAlias != nil {
		t.Fatalf("expected no alias when exhausted, got %v", *p2.Properties.TopicAlias)
	}
}

func TestApplyTopicAliasToClient_RevertReusesFreedAlias(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(2)
	m.ResetDownlink()

	p1 := &packets.Publish{Topic: "a/b", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p1)
	if p1.Properties == nil || p1.Properties.TopicAlias == nil || *p1.Properties.TopicAlias != 1 {
		t.Fatalf("expected first alias=1, got %+v", p1.Properties)
	}

	m.RevertDownlinkAlias("a/b")

	p2 := &packets.Publish{Topic: "c/d", Properties: &packets.PublishProperties{}}
	m.ApplyDownlink(p2)
	if p2.Properties == nil || p2.Properties.TopicAlias == nil {
		t.Fatalf("expected alias assigned after revert, got %+v", p2.Properties)
	}
	if got := *p2.Properties.TopicAlias; got != 1 {
		t.Fatalf("expected reverted alias to be reused as 1, got %d", got)
	}
}

func TestTopicAliasManager_ApplyDownlink_ConcurrentSameTopic(t *testing.T) {
	m := NewTopicAliasManager()
	m.SetDownlinkMax(10)
	m.ResetDownlink()

	const n = 32
	pubs := make([]*packets.Publish, 0, n)
	for i := 0; i < n; i++ {
		pubs = append(pubs, &packets.Publish{Topic: "a/b"})
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(p *packets.Publish) {
			defer wg.Done()
			m.ApplyDownlink(p)
		}(pubs[i])
	}
	wg.Wait()

	for i, p := range pubs {
		if p.Properties == nil || p.Properties.TopicAlias == nil {
			t.Fatalf("pub[%d] expected alias set, got nil properties=%v", i, p.Properties)
		}
		if *p.Properties.TopicAlias != 1 {
			t.Fatalf("pub[%d] expected alias=1, got %d", i, *p.Properties.TopicAlias)
		}
		// Topic may be kept for the first mapping establishment, or omitted on subsequent sends.
		if p.Topic != "" && p.Topic != "a/b" {
			t.Fatalf("pub[%d] expected topic empty or a/b, got %q", i, p.Topic)
		}
	}
}
