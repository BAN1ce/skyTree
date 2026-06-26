package domain

import (
	"context"
	"encoding/json"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

type ClientManager interface {
	ReadClient(id string) (interface{}, bool)
}

type SubCenter interface {
	GetShareGroupMembers(ctx context.Context, req *proto_topic.GetShareGroupMembersRequest) (*proto_topic.GetShareGroupMembersResponse, error)
	GetClientSubscriptions(ctx context.Context, req *proto_topic.GetClientSubscriptionsRequest) (*proto_topic.GetClientSubscriptionsResponse, error)
}

type SessionOwnerResolver interface {
	GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error)
}

type ResolvedClientOptions struct {
	DeliveryQoS         int
	SubscriptionIDsJSON string
	NoLocal             bool
	RAP                 bool
}

type OnlineShareGroupMember struct {
	ShareGroup  string
	ClientID    string
	TopicFilter string
	OwnerNodeID uint64
}

type SharedDeliveryStrategy interface {
	OnlineCandidates(ctx context.Context, cmd AssignTaskCommand) ([]*OnlineShareGroupMember, error)
	ResolveClientOptions(ctx context.Context, cmd ResolveClientOptionsCommand) (ResolvedClientOptions, bool, error)
}

type sharedDeliveryStrategy struct {
	subCenter            SubCenter
	sessionOwnerResolver SessionOwnerResolver
	clientManager        ClientManager
}

func NewSharedDeliveryStrategy(
	subCenter SubCenter,
	sessionOwnerResolver SessionOwnerResolver,
	clientManager ClientManager,
) SharedDeliveryStrategy {
	return &sharedDeliveryStrategy{
		subCenter:            subCenter,
		sessionOwnerResolver: sessionOwnerResolver,
		clientManager:        clientManager,
	}
}

func (s *sharedDeliveryStrategy) OnlineCandidates(ctx context.Context, cmd AssignTaskCommand) ([]*OnlineShareGroupMember, error) {
	if s == nil || s.subCenter == nil || cmd.ShareGroup == "" {
		return []*OnlineShareGroupMember{}, nil
	}
	resp, err := s.subCenter.GetShareGroupMembers(ctx, &proto_topic.GetShareGroupMembersRequest{
		ShareGroup: cmd.ShareGroup,
	})
	if err != nil || resp == nil {
		return nil, err
	}
	matched := make([]*proto_topic.ShareGroupMember, 0, len(resp.GetMembers()))
	clientIDs := make([]string, 0, len(resp.GetMembers()))
	for _, member := range resp.GetMembers() {
		if member == nil || member.GetClientID() == "" {
			continue
		}
		if cmd.TopicFilter != "" && member.GetTopicFilter() != cmd.TopicFilter {
			continue
		}
		matched = append(matched, member)
		clientIDs = append(clientIDs, member.GetClientID())
	}
	ownerNodes, err := s.resolveOnlineOwnerNodes(ctx, clientIDs)
	if err != nil {
		return nil, err
	}
	members := make([]*OnlineShareGroupMember, 0, len(matched))
	for _, member := range matched {
		ownerNodeID, ok := ownerNodes[member.GetClientID()]
		if !ok {
			continue
		}
		members = append(members, &OnlineShareGroupMember{
			ShareGroup:  cmd.ShareGroup,
			ClientID:    member.GetClientID(),
			TopicFilter: member.GetTopicFilter(),
			OwnerNodeID: ownerNodeID,
		})
	}
	return members, nil
}

func (s *sharedDeliveryStrategy) resolveOnlineOwnerNodes(ctx context.Context, clientIDs []string) (map[string]uint64, error) {
	ownerNodes := make(map[string]uint64, len(clientIDs))
	if len(clientIDs) == 0 {
		return ownerNodes, nil
	}
	if s.sessionOwnerResolver == nil {
		for _, clientID := range clientIDs {
			if clientID == "" {
				continue
			}
			if s.clientManager != nil {
				cli, ok := s.clientManager.ReadClient(clientID)
				if !ok || cli == nil {
					continue
				}
			}
			ownerNodes[clientID] = 0
		}
		return ownerNodes, nil
	}
	resp, err := s.sessionOwnerResolver.GetSessionOwners(
		ctx,
		&proto_session.ReadSessionOwnersRequest{ClientIDs: clientIDs},
	)
	if err != nil || resp == nil {
		return nil, err
	}
	for _, item := range resp.GetItems() {
		if item == nil || !item.GetExist() || item.GetClientID() == "" || item.GetOwner() == nil {
			continue
		}
		owner := item.GetOwner()
		if !owner.GetOnline() {
			continue
		}
		ownerNodes[item.GetClientID()] = owner.GetNodeID()
	}
	return ownerNodes, nil
}

func (s *sharedDeliveryStrategy) ResolveClientOptions(ctx context.Context, cmd ResolveClientOptionsCommand) (ResolvedClientOptions, bool, error) {
	if s == nil || s.subCenter == nil || cmd.ClientID == "" || cmd.ShareGroup == "" || cmd.TopicFilter == "" {
		return ResolvedClientOptions{}, false, nil
	}
	resp, err := s.subCenter.GetClientSubscriptions(ctx, &proto_topic.GetClientSubscriptionsRequest{ClientID: cmd.ClientID})
	if err != nil || resp == nil || len(resp.GetTopics()) == 0 {
		return ResolvedClientOptions{}, false, err
	}

	bestQoS := int32(-1)
	bestNoLocal := false
	bestRAP := true
	subIDs := make([]int32, 0, 2)
	for topicFilter, sub := range resp.GetTopics() {
		if sub == nil || !sharedsubscription.IsSharedSubscription(topicFilter) {
			continue
		}
		shareGroup, actualTopicFilter, parseErr := sharedsubscription.ParseSharedSubscription(topicFilter)
		if parseErr != nil || shareGroup != cmd.ShareGroup || actualTopicFilter != cmd.TopicFilter {
			continue
		}
		if cmd.PublisherClient != "" && cmd.PublisherClient == cmd.ClientID && sub.GetNoLocal() {
			continue
		}
		if sub.GetQoS() > bestQoS {
			bestQoS = sub.GetQoS()
			bestNoLocal = sub.GetNoLocal()
			bestRAP = sub.GetRetainAsPublished()
		}
		if sid := sub.GetSubscriptionIdentifier(); sid > 0 {
			subIDs = append(subIDs, sid)
		}
	}
	if bestQoS < 0 {
		return ResolvedClientOptions{}, false, nil
	}
	if len(subIDs) == 0 {
		subIDs = []int32{}
	}
	subIDsJSONBytes, err := json.Marshal(subIDs)
	if err != nil {
		return ResolvedClientOptions{}, false, err
	}
	deliveryQoS := int(bestQoS)
	if cmd.PublishQoS >= 0 && deliveryQoS > cmd.PublishQoS {
		deliveryQoS = cmd.PublishQoS
	}
	return ResolvedClientOptions{
		DeliveryQoS:         deliveryQoS,
		SubscriptionIDsJSON: string(subIDsJSONBytes),
		NoLocal:             bestNoLocal,
		RAP:                 bestRAP,
	}, true, nil
}
