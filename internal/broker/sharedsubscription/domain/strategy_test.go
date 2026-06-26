package domain

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

type fakeClientManager struct {
	online map[string]bool
}

func (m *fakeClientManager) ReadClient(id string) (interface{}, bool) {
	if !m.online[id] {
		return nil, false
	}
	return struct{}{}, true
}

type fakeSessionOwnerResolver struct {
	owners map[string]*proto_session.SessionOwner
}

func (r *fakeSessionOwnerResolver) GetSessionOwners(
	_ context.Context,
	req *proto_session.ReadSessionOwnersRequest,
) (*proto_session.ReadSessionOwnersResponse, error) {
	items := make([]*proto_session.ReadSessionOwnerItem, 0, len(req.GetClientIDs()))
	for _, clientID := range req.GetClientIDs() {
		owner := r.owners[clientID]
		items = append(items, &proto_session.ReadSessionOwnerItem{
			ClientID: clientID,
			Owner:    owner,
			Exist:    owner != nil,
		})
	}
	return &proto_session.ReadSessionOwnersResponse{Items: items}, nil
}

type fakeSubCenter struct {
	members []*proto_topic.ShareGroupMember
	topics  map[string]map[string]*proto_topic.SubOption
}

func (s *fakeSubCenter) GetShareGroupMembers(context.Context, *proto_topic.GetShareGroupMembersRequest) (*proto_topic.GetShareGroupMembersResponse, error) {
	return &proto_topic.GetShareGroupMembersResponse{Members: s.members}, nil
}

func (s *fakeSubCenter) GetClientSubscriptions(_ context.Context, req *proto_topic.GetClientSubscriptionsRequest) (*proto_topic.GetClientSubscriptionsResponse, error) {
	if req == nil {
		return &proto_topic.GetClientSubscriptionsResponse{Topics: map[string]*proto_topic.SubOption{}}, nil
	}
	return &proto_topic.GetClientSubscriptionsResponse{Topics: s.topics[req.GetClientID()]}, nil
}

func TestSharedDeliveryStrategyOnlineCandidates(t *testing.T) {
	strategy := NewSharedDeliveryStrategy(&fakeSubCenter{
		members: []*proto_topic.ShareGroupMember{
			{ClientID: "c1", TopicFilter: "a/b"},
			{ClientID: "c2", TopicFilter: "a/b"},
			{ClientID: "c3", TopicFilter: "a/c"},
		},
	}, &fakeSessionOwnerResolver{owners: map[string]*proto_session.SessionOwner{
		"c1": {ClientID: "c1", NodeID: 2, Online: true},
		"c2": {ClientID: "c2", NodeID: 3, Online: false},
	}}, nil)

	members, err := strategy.OnlineCandidates(context.Background(), AssignTaskCommand{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
	})
	if err != nil {
		t.Fatalf("OnlineCandidates error: %v", err)
	}
	if len(members) != 1 || members[0].ClientID != "c1" || members[0].OwnerNodeID != 2 {
		t.Fatalf("expected only c1 on node 2, got %+v", members)
	}
}

func TestSharedDeliveryStrategyResolveClientOptions(t *testing.T) {
	strategy := NewSharedDeliveryStrategy(&fakeSubCenter{
		topics: map[string]map[string]*proto_topic.SubOption{
			"c1": {
				"$share/g1/a/b": {
					Topic:                  "$share/g1/a/b",
					QoS:                    2,
					NoLocal:                false,
					RetainAsPublished:      true,
					SubscriptionIdentifier: 9,
				},
			},
		},
	}, nil, nil)

	options, ok, err := strategy.ResolveClientOptions(context.Background(), ResolveClientOptionsCommand{
		ClientID:    "c1",
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		PublishQoS:  1,
	})
	if err != nil {
		t.Fatalf("ResolveClientOptions error: %v", err)
	}
	if !ok {
		t.Fatalf("expected options resolved")
	}
	if options.DeliveryQoS != 1 {
		t.Fatalf("expected qos clamp to 1, got %d", options.DeliveryQoS)
	}
	if options.SubscriptionIDsJSON != "[9]" {
		t.Fatalf("expected subscription ids [9], got %s", options.SubscriptionIDsJSON)
	}
}
