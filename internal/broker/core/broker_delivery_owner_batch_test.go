package core

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

type recordingOwnerLookupSessionCenter struct {
	*timeoutSessionCenter
	owners       map[string]*proto_session.SessionOwner
	batchCalls   int
	singleCalls  int
	batchQueries [][]string
}

func (c *recordingOwnerLookupSessionCenter) GetSessionOwner(_ context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	c.singleCalls++
	if c.owners == nil {
		return &proto_session.ReadSessionOwnerResponse{Exist: false}, nil
	}
	owner := c.owners[request.GetClientID()]
	if owner == nil {
		return &proto_session.ReadSessionOwnerResponse{Exist: false}, nil
	}
	return &proto_session.ReadSessionOwnerResponse{Exist: true, Owner: owner}, nil
}

func (c *recordingOwnerLookupSessionCenter) GetSessionOwners(_ context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	c.batchCalls++
	ids := append([]string(nil), request.GetClientIDs()...)
	c.batchQueries = append(c.batchQueries, ids)

	seen := make(map[string]struct{}, len(request.GetClientIDs()))
	items := make([]*proto_session.ReadSessionOwnerItem, 0, len(request.GetClientIDs()))
	for _, clientID := range request.GetClientIDs() {
		if clientID == "" {
			continue
		}
		if _, ok := seen[clientID]; ok {
			continue
		}
		seen[clientID] = struct{}{}
		owner := c.owners[clientID]
		items = append(items, &proto_session.ReadSessionOwnerItem{
			ClientID: clientID,
			Owner:    owner,
			Exist:    owner != nil,
		})
	}
	return &proto_session.ReadSessionOwnersResponse{Items: items}, nil
}

type notifyCall struct {
	nodeID       uint64
	publishTopic string
	clientIDs    []string
	kind         deliveryevent.Kind
}

type recordingDeliveryEvent struct {
	calls        []notifyCall
	errByNode    map[uint64]error
	errSeqByNode map[uint64][]error
}

func (e *recordingDeliveryEvent) AddListener(context.Context, string, delivery_notify.NotifyHandler) (string, string, error) {
	return "", "", nil
}

func (e *recordingDeliveryEvent) DeleteListener(context.Context, string, string) error {
	return nil
}

func (e *recordingDeliveryEvent) NotifyToNode(_ context.Context, nodeID uint64, publishTopic string, clientIDs []string, kind deliveryevent.Kind, payload []byte, clientOptions map[string]delivery_notify.ClientDeliveryOptions) error {
	_ = payload
	_ = clientOptions
	copied := append([]string(nil), clientIDs...)
	e.calls = append(e.calls, notifyCall{
		nodeID:       nodeID,
		publishTopic: publishTopic,
		clientIDs:    copied,
		kind:         kind,
	})
	if e.errSeqByNode != nil && len(e.errSeqByNode[nodeID]) > 0 {
		err := e.errSeqByNode[nodeID][0]
		e.errSeqByNode[nodeID] = e.errSeqByNode[nodeID][1:]
		return err
	}
	if e.errByNode != nil && e.errByNode[nodeID] != nil {
		return e.errByNode[nodeID]
	}
	return nil
}

func (e *recordingDeliveryEvent) NotifySharedWake(context.Context, delivery_notify.SharedWakePayload) error {
	return nil
}

type sharedMembersSubCenter struct {
	brokerExpirySubCenter
	membersByGroup map[string][]*proto_topic.ShareGroupMember
}

func (s *sharedMembersSubCenter) GetShareGroupMembers(_ context.Context, req *proto_topic.GetShareGroupMembersRequest) (*proto_topic.GetShareGroupMembersResponse, error) {
	members := s.membersByGroup[req.GetShareGroup()]
	return &proto_topic.GetShareGroupMembersResponse{Members: members}, nil
}

func TestWakeClientDeliveryRunnersUsesBatchOwnerLookup(t *testing.T) {
	center := &recordingOwnerLookupSessionCenter{
		timeoutSessionCenter: &timeoutSessionCenter{},
		owners: map[string]*proto_session.SessionOwner{
			"c1": {ClientID: "c1", NodeID: 101, Online: true},
			"c2": {ClientID: "c2", NodeID: 202, Online: true},
		},
	}
	event := &recordingDeliveryEvent{}
	b := &Broker{
		state:    brokerStateCenters{sessionCenter: center},
		delivery: brokerDeliveryResources{event: event},
	}

	b.wakeClientDeliveryRunners(context.Background(), "topic/a", []delivery.ClientPlan{
		{ClientID: "c1"},
		{ClientID: "c2"},
		{ClientID: "c1"},
	})

	if center.batchCalls != 1 {
		t.Fatalf("expected one batch owner lookup, got %d", center.batchCalls)
	}
	if center.singleCalls != 0 {
		t.Fatalf("expected zero single owner lookup, got %d", center.singleCalls)
	}
	if len(center.batchQueries) != 1 {
		t.Fatalf("expected one batch query, got %d", len(center.batchQueries))
	}
	if got := center.batchQueries[0]; len(got) != 2 || got[0] != "c1" || got[1] != "c2" {
		t.Fatalf("expected deduped query [c1 c2], got %v", got)
	}
	if len(event.calls) != 2 {
		t.Fatalf("expected two wake notifications, got %d", len(event.calls))
	}
	for _, call := range event.calls {
		if call.kind != deliveryevent.KindWake {
			t.Fatalf("expected wake kind, got %v", call.kind)
		}
		if call.publishTopic != "topic/a" {
			t.Fatalf("unexpected topic %q", call.publishTopic)
		}
		if len(call.clientIDs) != 1 {
			t.Fatalf("expected one client per node, got %v", call.clientIDs)
		}
	}
}

func TestWakeClientDeliveryRunnersRecordsWakeMetrics(t *testing.T) {
	center := &recordingOwnerLookupSessionCenter{
		timeoutSessionCenter: &timeoutSessionCenter{},
		owners: map[string]*proto_session.SessionOwner{
			"local-client":  {ClientID: "local-client", NodeID: 7, Online: true},
			"remote-client": {ClientID: "remote-client", NodeID: 8, Online: true},
		},
	}
	eventErr := errors.New("notify failed")
	event := &recordingDeliveryEvent{errByNode: map[uint64]error{8: eventErr}}
	b := &Broker{
		state:    brokerStateCenters{sessionCenter: center},
		delivery: brokerDeliveryResources{event: event},
		cluster: brokerClusterResources{
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{LocalNodeID: 7}},
		},
	}

	localBefore := histogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "normal",
		"mode":   "event_local",
		"result": "success",
	})
	remoteErrorBefore := histogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "normal",
		"mode":   "event_remote",
		"result": "error",
	})

	b.wakeClientDeliveryRunners(context.Background(), "topic/a", []delivery.ClientPlan{
		{ClientID: "local-client"},
		{ClientID: "remote-client"},
	})

	assertHistogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "normal",
		"mode":   "event_local",
		"result": "success",
	}, localBefore+1)
	assertHistogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "normal",
		"mode":   "event_remote",
		"result": "error",
	}, remoteErrorBefore+normalDeliveryWakeMaxAttempts)
}

func TestWakeClientDeliveryRunnersRetriesTransientNotifyFailure(t *testing.T) {
	center := &recordingOwnerLookupSessionCenter{
		timeoutSessionCenter: &timeoutSessionCenter{},
		owners: map[string]*proto_session.SessionOwner{
			"remote-client": {ClientID: "remote-client", NodeID: 8, Online: true},
		},
	}
	event := &recordingDeliveryEvent{
		errSeqByNode: map[uint64][]error{
			8: {errors.New("transient notify failed"), nil},
		},
	}
	b := &Broker{
		state:    brokerStateCenters{sessionCenter: center},
		delivery: brokerDeliveryResources{event: event},
		cluster: brokerClusterResources{
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{LocalNodeID: 7}},
		},
	}

	b.wakeClientDeliveryRunners(context.Background(), "topic/a", []delivery.ClientPlan{
		{ClientID: "remote-client"},
	})

	if len(event.calls) != 2 {
		t.Fatalf("expected retry after transient wake failure, got %d calls", len(event.calls))
	}
	for _, call := range event.calls {
		if call.nodeID != 8 {
			t.Fatalf("expected retry to target node 8, got %d", call.nodeID)
		}
		if len(call.clientIDs) != 1 || call.clientIDs[0] != "remote-client" {
			t.Fatalf("unexpected retry client IDs: %v", call.clientIDs)
		}
	}
}

func TestNotifyQoS0DirectNormalPlansUsesBatchOwnerLookup(t *testing.T) {
	center := &recordingOwnerLookupSessionCenter{
		timeoutSessionCenter: &timeoutSessionCenter{},
		owners: map[string]*proto_session.SessionOwner{
			"c1": {ClientID: "c1", NodeID: 77, Online: true},
			"c2": {ClientID: "c2", NodeID: 77, Online: true},
		},
	}
	event := &recordingDeliveryEvent{}
	b := &Broker{
		state:    brokerStateCenters{sessionCenter: center},
		delivery: brokerDeliveryResources{event: event},
	}

	b.notifyQoS0DirectNormalPlans(
		context.Background(),
		&packets.Publish{Topic: "topic/b"},
		[]delivery.ClientPlan{{ClientID: "c1"}, {ClientID: "c2"}},
		[]byte("payload"),
		newOwnerNodeLookupCache(),
	)

	if center.batchCalls != 1 {
		t.Fatalf("expected one batch owner lookup, got %d", center.batchCalls)
	}
	if center.singleCalls != 0 {
		t.Fatalf("expected zero single owner lookup, got %d", center.singleCalls)
	}
	if len(event.calls) != 1 {
		t.Fatalf("expected one grouped notification, got %d", len(event.calls))
	}
	call := event.calls[0]
	if call.kind != deliveryevent.KindQoS0Direct {
		t.Fatalf("expected qos0 direct kind, got %v", call.kind)
	}
	if call.nodeID != 77 {
		t.Fatalf("expected node 77, got %d", call.nodeID)
	}
	sort.Strings(call.clientIDs)
	if len(call.clientIDs) != 2 || call.clientIDs[0] != "c1" || call.clientIDs[1] != "c2" {
		t.Fatalf("expected clientIDs [c1 c2], got %v", call.clientIDs)
	}
}

func TestOnlineQoS0SharedCandidatesReuseOwnerCacheAcrossTasks(t *testing.T) {
	center := &recordingOwnerLookupSessionCenter{
		timeoutSessionCenter: &timeoutSessionCenter{},
		owners: map[string]*proto_session.SessionOwner{
			"c1": {ClientID: "c1", NodeID: 9, Online: true},
		},
	}
	subCenter := &sharedMembersSubCenter{
		membersByGroup: map[string][]*proto_topic.ShareGroupMember{
			"g1": {{ClientID: "c1", TopicFilter: "topic/s"}},
			"g2": {{ClientID: "c1", TopicFilter: "topic/s"}},
		},
	}
	b := &Broker{
		state: brokerStateCenters{
			sessionCenter: center,
			subCenter:     subCenter,
		},
	}
	cache := newOwnerNodeLookupCache()

	candidates1 := b.onlineQoS0SharedCandidates(context.Background(), delivery.ShareGroupTask{ShareGroup: "g1", TopicFilter: "topic/s"}, cache)
	candidates2 := b.onlineQoS0SharedCandidates(context.Background(), delivery.ShareGroupTask{ShareGroup: "g2", TopicFilter: "topic/s"}, cache)

	if len(candidates1) != 1 || len(candidates2) != 1 {
		t.Fatalf("expected one candidate per group, got %v and %v", candidates1, candidates2)
	}
	if center.batchCalls != 1 {
		t.Fatalf("expected cache to reuse owner lookup across tasks, got %d batch calls", center.batchCalls)
	}
	if center.singleCalls != 0 {
		t.Fatalf("expected zero single owner lookup, got %d", center.singleCalls)
	}
}
