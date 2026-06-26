package selector

import (
	"testing"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestRoundRobinSelectorCyclesMembers(t *testing.T) {
	s := NewRoundRobinSelector()
	members := []*sharedsubscription.ShareGroupMember{
		{ClientID: "a"},
		{ClientID: "b"},
	}

	if got := s.Select("g", members, nil); got != "a" {
		t.Fatalf("first pick should be a, got %q", got)
	}
	if got := s.Select("g", members, nil); got != "b" {
		t.Fatalf("second pick should be b, got %q", got)
	}
	if got := s.Select("g", members, nil); got != "a" {
		t.Fatalf("third pick should wrap to a, got %q", got)
	}
}

func TestRoundRobinSelectorSkipsInvalidMembers(t *testing.T) {
	s := NewRoundRobinSelector()
	members := []*sharedsubscription.ShareGroupMember{
		nil,
		{ClientID: ""},
		{ClientID: "ok"},
	}
	if got := s.Select("g", members, nil); got != "ok" {
		t.Fatalf("expected only valid member selected, got %q", got)
	}
}

func TestHashSelectorDeterministicForSameMessage(t *testing.T) {
	s := NewHashSelector()
	members := []*sharedsubscription.ShareGroupMember{
		{ClientID: "a"},
		{ClientID: "b"},
		{ClientID: "c"},
	}
	pub := &packets.Publish{Topic: "t/1", Payload: []byte("payload"), PacketID: 10}

	first := s.Select("g", members, pub)
	second := s.Select("g", members, pub)
	if first == "" {
		t.Fatal("expected non-empty selection")
	}
	if second != first {
		t.Fatalf("expected deterministic selection, got %q then %q", first, second)
	}
}
