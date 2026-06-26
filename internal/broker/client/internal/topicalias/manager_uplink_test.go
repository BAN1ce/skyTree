package topicalias

import (
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestApplyTopicAliasFromClient_SetAndGet(t *testing.T) {
	m := NewTopicAliasManager()

	alias := uint16(7)
	p1 := &packets.Publish{
		Topic:      "a/b",
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}
	finalTopic, updated, err := m.ApplyUplink(p1, 10)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if updated {
		t.Fatalf("expected updated=false when TopicName present")
	}
	if finalTopic != "a/b" {
		t.Fatalf("expected finalTopic=a/b, got %q", finalTopic)
	}
	if p1.Properties != nil && p1.Properties.TopicAlias != nil {
		t.Fatalf("expected inbound topic alias to be normalized away, got %v", *p1.Properties.TopicAlias)
	}

	// Now publish with empty topic, alias only.
	p2 := &packets.Publish{
		Topic:      "",
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}
	finalTopic2, updated2, err := m.ApplyUplink(p2, 10)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !updated2 {
		t.Fatalf("expected updated=true when TopicName resolved from alias")
	}
	if finalTopic2 != "a/b" {
		t.Fatalf("expected finalTopic=a/b, got %q", finalTopic2)
	}
	if p2.Topic != "a/b" {
		t.Fatalf("expected packet topic to be normalized to resolved topic, got %q", p2.Topic)
	}
	if p2.Properties != nil && p2.Properties.TopicAlias != nil {
		t.Fatalf("expected resolved inbound topic alias to be stripped, got %v", *p2.Properties.TopicAlias)
	}
}

func TestApplyTopicAliasFromClient_InvalidAlias(t *testing.T) {
	m := NewTopicAliasManager()

	zero := uint16(0)
	p := &packets.Publish{
		Topic:      "a/b",
		Properties: &packets.PublishProperties{TopicAlias: &zero},
	}
	_, _, err := m.ApplyUplink(p, 10)
	if err != ErrTopicAliasInvalid {
		t.Fatalf("expected ErrTopicAliasInvalid, got %v", err)
	}
}

func TestApplyTopicAliasFromClient_AliasNotFound(t *testing.T) {
	m := NewTopicAliasManager()

	alias := uint16(1)
	p := &packets.Publish{
		Topic:      "",
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}
	_, _, err := m.ApplyUplink(p, 10)
	if err != ErrTopicAliasNotFound {
		t.Fatalf("expected ErrTopicAliasNotFound, got %v", err)
	}
}

func TestApplyTopicAliasFromClient_AliasExceedsMaximum(t *testing.T) {
	m := NewTopicAliasManager()

	alias := uint16(11)
	p := &packets.Publish{
		Topic:      "a/b",
		Properties: &packets.PublishProperties{TopicAlias: &alias},
	}
	_, _, err := m.ApplyUplink(p, 10)
	if err != ErrTopicAliasInvalid {
		t.Fatalf("expected ErrTopicAliasInvalid, got %v", err)
	}
}
