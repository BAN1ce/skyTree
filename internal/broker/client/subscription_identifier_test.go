package client

import (
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestSetPublishSubscriptionIdentifiersFiltersInvalidValues(t *testing.T) {
	publish := &packets.Publish{}
	setPublishSubscriptionIdentifiers(publish, []int32{-1, 0, 1, 2, 268435455, 268435456})
	if publish.Properties == nil {
		t.Fatal("expected publish properties")
	}
	got := publish.Properties.SubscriptionIdentifier
	if len(got) != 3 {
		t.Fatalf("expected 3 valid subscription identifiers, got %d (%v)", len(got), got)
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 268435455 {
		t.Fatalf("unexpected subscription identifiers: %v", got)
	}
}

func TestSetPublishSubscriptionIdentifiersClearsWhenNoValidValue(t *testing.T) {
	publish := &packets.Publish{
		Properties: &packets.PublishProperties{
			SubscriptionIdentifier: []int{7},
		},
	}
	setPublishSubscriptionIdentifiers(publish, []int32{0, -2, 268435456})
	if publish.Properties == nil {
		t.Fatal("expected properties to remain allocated")
	}
	if publish.Properties.SubscriptionIdentifier != nil {
		t.Fatalf("expected subscription identifiers cleared, got %v", publish.Properties.SubscriptionIdentifier)
	}
}
