package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	subcenter "github.com/BAN1ce/skyTree/internal/broker/subcenter/memory"
	broker_sub "github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

func TestSubCenterRouter_MultiMatch_SubIDsAggregated(t *testing.T) {
	// Verify that overlapping subscriptions do not collapse into a single SubOption in V2 routing.
	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()

	// client subscribes two overlapping filters with different SubscriptionIdentifier.
	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a/#", QoS: 0, SubscriptionIdentifier: 1},
			{Topic: "a/b", QoS: 1, SubscriptionIdentifier: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	res, err := r.Route(ctx, &packets.Publish{Topic: "a/b", QoS: 1}, "publisher")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if res == nil || len(res.Plans) != 1 {
		t.Fatalf("expected 1 plan, got %#v", res)
	}
	if res.Plans[0].ClientID != "c1" {
		t.Fatalf("expected client c1, got %q", res.Plans[0].ClientID)
	}
	if res.Plans[0].DeliveryQoS != 1 {
		t.Fatalf("expected max QoS 1, got %d", res.Plans[0].DeliveryQoS)
	}
	var ids []int32
	if err := json.Unmarshal([]byte(res.Plans[0].SubscriptionIDsJSON), &ids); err != nil {
		t.Fatalf("invalid subscription_ids json: %v (%q)", err, res.Plans[0].SubscriptionIDsJSON)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("expected [1,2], got %v", ids)
	}
}

func TestSubCenterRouter_SharedAndNormal_ProducesPlanAndShareTask(t *testing.T) {
	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()

	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a", QoS: 1, SubscriptionIdentifier: 11},
			{Topic: "$share/g/a", QoS: 1, SubscriptionIdentifier: 22},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	res, err := r.Route(ctx, &packets.Publish{Topic: "a", QoS: 1}, "publisher")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected result, got nil")
	} // Normal plan should exist.
	foundPlan := false
	for _, p := range res.Plans {
		if p.ClientID != "c1" {
			continue
		}
		foundPlan = true
		var ids []int32
		if err := json.Unmarshal([]byte(p.SubscriptionIDsJSON), &ids); err != nil {
			t.Fatalf("invalid plan subscription_ids json: %v (%q)", err, p.SubscriptionIDsJSON)
		}
		if len(ids) != 1 || ids[0] != 11 {
			t.Fatalf("expected plan subIDs [11], got %v", ids)
		}
	}
	if !foundPlan {
		t.Fatalf("expected normal plan for c1, got %#v", res.Plans)
	}

	// Share task should exist.
	foundShare := false
	for _, st := range res.ShareGroupTasks {
		if st.ShareGroup != "g" || st.TopicFilter != "a" {
			continue
		}
		foundShare = true
		var ids []int32
		if err := json.Unmarshal([]byte(st.SubscriptionIDs), &ids); err != nil {
			t.Fatalf("invalid share subscription_ids json: %v (%q)", err, st.SubscriptionIDs)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		if len(ids) != 1 || ids[0] != 22 {
			t.Fatalf("expected share subIDs [22], got %v", ids)
		}
	}
	if !foundShare {
		t.Fatalf("expected share task for g/a, got %#v", res.ShareGroupTasks)
	}
}

func TestSubCenterRouter_NoLocalFiltersOnlySelfMatches(t *testing.T) {
	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()

	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a/#", QoS: 2, NoLocal: true, SubscriptionIdentifier: 10},
			{Topic: "a/b", QoS: 1, NoLocal: false, SubscriptionIdentifier: 20},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	res, err := r.Route(ctx, &packets.Publish{Topic: "a/b", QoS: 2}, "c1")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if res == nil || len(res.Plans) != 1 {
		t.Fatalf("expected one remaining plan, got %#v", res)
	}
	plan := res.Plans[0]
	if plan.ClientID != "c1" {
		t.Fatalf("expected client c1, got %q", plan.ClientID)
	}
	if plan.DeliveryQoS != 1 {
		t.Fatalf("expected delivery QoS from remaining subscription, got %d", plan.DeliveryQoS)
	}
	if plan.WinnerNoLocal {
		t.Fatal("remaining winner should not be NoLocal")
	}
	var ids []int32
	if err := json.Unmarshal([]byte(plan.SubscriptionIDsJSON), &ids); err != nil {
		t.Fatalf("invalid subscription ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != 20 {
		t.Fatalf("expected only remaining subscription id [20], got %v", ids)
	}
}

func TestSubCenterRouter_NoLocalDropsPlanWhenAllSelfMatchesFiltered(t *testing.T) {
	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()

	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a/#", QoS: 1, NoLocal: true, SubscriptionIdentifier: 10},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	res, err := r.Route(ctx, &packets.Publish{Topic: "a/b", QoS: 1}, "c1")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil route result")
	}
	if len(res.Plans) != 0 {
		t.Fatalf("expected no plan after NoLocal filtering, got %#v", res.Plans)
	}
}

func TestSubCenterRouter_ReturnsErrorWhenPlanMarshalFails(t *testing.T) {
	original := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() {
		jsonMarshal = original
	})

	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()
	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "a/#", QoS: 1, SubscriptionIdentifier: 10},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	_, err = r.Route(ctx, &packets.Publish{Topic: "a/b", QoS: 1}, "publisher")
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "client c1") || !strings.Contains(err.Error(), "topic a/b") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubCenterRouter_ReturnsErrorWhenShareTaskMarshalFails(t *testing.T) {
	original := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() {
		jsonMarshal = original
	})

	ctx := context.Background()
	var sc broker_sub.Center = subcenter.NewMemorySubCenter()
	_, err := sc.CreateSub(ctx, &proto.SubRequest{
		ClientID: "c1",
		Topics: []*proto.SubOption{
			{Topic: "$share/g/a/#", QoS: 1, SubscriptionIdentifier: 22},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub failed: %v", err)
	}

	r := NewSubCenterRouter(sc)
	_, err = r.Route(ctx, &packets.Publish{Topic: "a/b", QoS: 1}, "publisher")
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "share_group=g") || !strings.Contains(err.Error(), "topic=a/b") {
		t.Fatalf("unexpected error: %v", err)
	}
}
