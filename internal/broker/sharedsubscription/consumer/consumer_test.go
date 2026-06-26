package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/domain"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription/selector"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type staticSelector struct {
	clientID string
}

func (s staticSelector) Select(_ string, _ []*sharedsubscription.ShareGroupMember, _ *packets.Publish) string {
	return s.clientID
}

type recordingSelector struct {
	members []*sharedsubscription.ShareGroupMember
}

func (s *recordingSelector) Select(_ string, members []*sharedsubscription.ShareGroupMember, _ *packets.Publish) string {
	s.members = append([]*sharedsubscription.ShareGroupMember(nil), members...)
	if len(members) == 0 {
		return ""
	}
	return members[0].ClientID
}

type fakeSharedStrategy struct {
	members []*domain.OnlineShareGroupMember
	options domain.ResolvedClientOptions
}

func (s fakeSharedStrategy) OnlineCandidates(context.Context, domain.AssignTaskCommand) ([]*domain.OnlineShareGroupMember, error) {
	return s.members, nil
}

func (s fakeSharedStrategy) ResolveClientOptions(
	context.Context,
	domain.ResolveClientOptionsCommand,
) (domain.ResolvedClientOptions, bool, error) {
	return s.options, true, nil
}

type recordingDeliveryEvent struct {
	calls        int
	nodeID       uint64
	publishTopic string
	clientIDs    []string
	kind         deliveryevent.Kind
}

func (e *recordingDeliveryEvent) AddListener(
	context.Context,
	string,
	delivery_notify.NotifyHandler,
) (string, string, error) {
	return "", "", nil
}

func (e *recordingDeliveryEvent) DeleteListener(context.Context, string, string) error {
	return nil
}

func (e *recordingDeliveryEvent) NotifyToNode(
	_ context.Context,
	nodeID uint64,
	publishTopic string,
	clientIDs []string,
	kind deliveryevent.Kind,
	_ []byte,
	_ map[string]delivery_notify.ClientDeliveryOptions,
) error {
	e.calls++
	e.nodeID = nodeID
	e.publishTopic = publishTopic
	e.clientIDs = append([]string(nil), clientIDs...)
	e.kind = kind
	return nil
}

func (e *recordingDeliveryEvent) NotifySharedWake(context.Context, delivery_notify.SharedWakePayload) error {
	return nil
}

type fakeTaskStore struct {
	calls    int
	lastPlan delivery.ClientPlan
}

func (s *fakeTaskStore) SavePublishMessage(context.Context, time.Time, string, *packets.Publish, uuid.UUID) (uuid.UUID, error) {
	panic("not used")
}

func (s *fakeTaskStore) AppendClientTask(ctx context.Context, ts time.Time, clientID string, messageID uuid.UUID, plan delivery.ClientPlan) (uuid.UUID, bool, error) {
	s.calls++
	s.lastPlan = plan
	return uuid.New(), true, nil
}

type fakeSharedStore struct {
	markCalls int
}

func (f *fakeSharedStore) EnsureSchema(context.Context) error { return nil }
func (f *fakeSharedStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	panic("not used")
}
func (f *fakeSharedStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	f.markCalls++
	return nil
}
func (f *fakeSharedStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	return true, nil
}
func (f *fakeSharedStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	panic("not used")
}
func (f *fakeSharedStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	// This should NOT be called anymore; shared + normal should produce two tasks.
	panic("unexpected call: CheckDeliveryTaskExists")
}
func (f *fakeSharedStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	return &sharedsubscription.ShareGroupCursor{}, nil
}
func (f *fakeSharedStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	return nil
}
func (f *fakeSharedStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}

type fakeSubCenter struct {
	members    []*proto.ShareGroupMember
	shareGroup string
	clientSubs map[string]map[string]*proto.SubOption
}

func (f *fakeSubCenter) CreateSub(context.Context, *proto.SubRequest) (*proto.SubResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) DeleteSub(context.Context, *proto.UnSubRequest) (*proto.UnSubResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetAllMatchTopics(context.Context, *proto.GetAllMatchTopicsRequest) (*proto.GetAllMatchTopicsResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetAllMatchTopicsForWildTopic(context.Context, *proto.GetAllMatchTopicsForWildTopicRequest) (*proto.GetAllMatchTopicsForWildTopicResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) DeleteClient(context.Context, *proto.DeleteClientRequest) (*proto.DeleteClientResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) DeleteTopic(context.Context, *proto.DeleteTopicRequest) (*proto.DeleteTopicResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetAllMatchClient(context.Context, *proto.GetAllSubTopicClientRequest) (*proto.GetAllSubTopicClientResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetAllMatchClientV2(context.Context, *proto.GetAllMatchClientV2Request) (*proto.GetAllMatchClientV2Response, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetSubTree(context.Context, *proto.GetSubTreeRequest) (*proto.GetSubTreeResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) SetClientOwnerToken(context.Context, *proto.SetClientOwnerTokenRequest) (*proto.SetClientOwnerTokenResponse, error) {
	panic("not used")
}
func (f *fakeSubCenter) GetClientSubscriptions(_ context.Context, req *proto.GetClientSubscriptionsRequest) (*proto.GetClientSubscriptionsResponse, error) {
	if req == nil || req.GetClientID() == "" {
		return &proto.GetClientSubscriptionsResponse{Topics: map[string]*proto.SubOption{}}, nil
	}
	if f.clientSubs != nil {
		if topics, ok := f.clientSubs[req.GetClientID()]; ok {
			return &proto.GetClientSubscriptionsResponse{Topics: topics}, nil
		}
	}
	shareGroup := f.shareGroup
	if shareGroup == "" {
		shareGroup = "g"
	}
	topics := make(map[string]*proto.SubOption)
	for _, mem := range f.members {
		if mem == nil || mem.GetClientID() != req.GetClientID() || mem.GetTopicFilter() == "" {
			continue
		}
		filter := "$share/" + shareGroup + "/" + mem.GetTopicFilter()
		topics[filter] = &proto.SubOption{
			Topic:                  filter,
			QoS:                    1,
			RetainAsPublished:      true,
			NoLocal:                false,
			SubscriptionIdentifier: 1,
		}
	}
	return &proto.GetClientSubscriptionsResponse{Topics: topics}, nil
}
func (f *fakeSubCenter) GetShareGroupMembers(context.Context, *proto.GetShareGroupMembersRequest) (*proto.GetShareGroupMembersResponse, error) {
	return &proto.GetShareGroupMembersResponse{Members: f.members}, nil
}

func TestSharedConsumer_ProcessTask_DoesNotDedupeByMessageID(t *testing.T) {
	// Regression test for shared+normal duplicates; shared consumer must not skip enqueue
	// just because the same messageID already exists for the client in delivery_task.
	_ = store.SharedSubscriptionStore(&fakeSharedStore{}) // compile-time guard
	_ = selector.ShareGroupSelector(staticSelector{})     // compile-time guard

	ts := &fakeTaskStore{}
	c := &SharedSubscriptionConsumer{
		shareGroup:    "g",
		topicFilter:   "a",
		store:         &fakeSharedStore{},
		taskStore:     ts,
		clientManager: nil, // include all members
		selector:      staticSelector{clientID: "c1"},
		subCenter:     &fakeSubCenter{members: []*proto.ShareGroupMember{{ClientID: "c1", TopicFilter: "a"}}},
		ctx:           context.Background(),
		wakeClientFunc: func(string) {
		},
	}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "g",
		TopicFilter: "a",
		MessageID:   uuid.New(),
		DeliveryQoS: 0,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}

	if err := c.processTask(task); err != nil {
		t.Fatalf("processTask error: %v", err)
	}
	if ts.calls != 1 {
		t.Fatalf("expected AppendClientTask called once, got %d", ts.calls)
	}
}

func TestSharedConsumer_ProcessTask_RecordsDeliveryMetrics(t *testing.T) {
	ts := &fakeTaskStore{}
	c := &SharedSubscriptionConsumer{
		shareGroup:     "g",
		topicFilter:    "a",
		store:          &fakeSharedStore{},
		taskStore:      ts,
		clientManager:  nil,
		selector:       staticSelector{clientID: "c1"},
		subCenter:      &fakeSubCenter{members: []*proto.ShareGroupMember{{ClientID: "c1", TopicFilter: "a"}}},
		ctx:            context.Background(),
		wakeClientFunc: func(string) {},
	}
	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "g",
		TopicFilter: "a",
		MessageID:   uuid.New(),
		DeliveryQoS: 1,
		PublishQoS:  1,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}
	enqueueBefore := consumerHistogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "shared",
		"qos":    "1",
		"result": "success",
	})
	wakeBefore := consumerHistogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "shared",
		"mode":   "event_local",
		"result": "success",
	})

	if err := c.processTask(task); err != nil {
		t.Fatalf("processTask error: %v", err)
	}

	assertConsumerHistogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "shared",
		"qos":    "1",
		"result": "success",
	}, enqueueBefore+1)
	assertConsumerHistogramSampleCount(t, metric.DeliveryWakeDelaySeconds, map[string]string{
		"path":   "shared",
		"mode":   "event_local",
		"result": "success",
	}, wakeBefore+1)
}

func TestSharedConsumer_ProcessTask_WakesRemoteOwner(t *testing.T) {
	ts := &fakeTaskStore{}
	ev := &recordingDeliveryEvent{}
	c := &SharedSubscriptionConsumer{
		shareGroup:  "g",
		topicFilter: "a",
		store:       &fakeSharedStore{},
		taskStore:   ts,
		selector:    staticSelector{clientID: "remote"},
		strategy: fakeSharedStrategy{
			members: []*domain.OnlineShareGroupMember{
				{ShareGroup: "g", ClientID: "remote", TopicFilter: "a", OwnerNodeID: 2},
			},
			options: domain.ResolvedClientOptions{DeliveryQoS: 1, RAP: true},
		},
		ctx:           context.Background(),
		localNodeID:   1,
		deliveryEvent: ev,
	}
	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "g",
		TopicFilter: "a",
		MessageID:   uuid.New(),
		DeliveryQoS: 1,
		PublishQoS:  1,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}

	if err := c.processTask(task); err != nil {
		t.Fatalf("processTask error: %v", err)
	}
	if ts.calls != 1 || ts.lastPlan.ClientID != "remote" {
		t.Fatalf("expected remote client task, calls=%d plan=%+v", ts.calls, ts.lastPlan)
	}
	if ev.calls != 1 || ev.nodeID != 2 || ev.kind != deliveryevent.KindWake {
		t.Fatalf("expected one remote wake to node 2, got calls=%d node=%d kind=%d", ev.calls, ev.nodeID, ev.kind)
	}
	if len(ev.clientIDs) != 1 || ev.clientIDs[0] != "remote" {
		t.Fatalf("expected remote client wake, got %+v", ev.clientIDs)
	}
}

func TestSharedConsumer_ProcessTask_UsesTaskOffsetForCandidateOrder(t *testing.T) {
	ts := &fakeTaskStore{}
	c := &SharedSubscriptionConsumer{
		shareGroup: "g",
		store:      &fakeSharedStore{},
		taskStore:  ts,
		strategy: fakeSharedStrategy{
			members: []*domain.OnlineShareGroupMember{
				{ShareGroup: "g", ClientID: "c1", TopicFilter: "a", OwnerNodeID: 1},
				{ShareGroup: "g", ClientID: "c2", TopicFilter: "a", OwnerNodeID: 1},
				{ShareGroup: "g", ClientID: "c3", TopicFilter: "a", OwnerNodeID: 1},
			},
			options: domain.ResolvedClientOptions{DeliveryQoS: 1, RAP: true},
		},
		ctx:            context.Background(),
		localNodeID:    1,
		wakeClientFunc: func(string) {},
	}
	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "g",
		TopicFilter: "a",
		MessageID:   uuid.New(),
		DeliveryQoS: 1,
		PublishQoS:  1,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}

	if err := c.processTaskAt(task, 2); err != nil {
		t.Fatalf("processTask error: %v", err)
	}
	if ts.calls != 1 || ts.lastPlan.ClientID != "c3" {
		t.Fatalf("expected task offset 2 to select c3, calls=%d plan=%+v", ts.calls, ts.lastPlan)
	}
}

func TestSharedConsumer_ProcessTask_FiltersMembersByTopicFilter(t *testing.T) {
	ts := &fakeTaskStore{}
	c := &SharedSubscriptionConsumer{
		shareGroup:     "g",
		topicFilter:    "a/#",
		store:          &fakeSharedStore{},
		taskStore:      ts,
		clientManager:  nil,
		selector:       staticSelector{clientID: "c1"},
		subCenter:      &fakeSubCenter{members: []*proto.ShareGroupMember{{ClientID: "c1", TopicFilter: "a/#"}, {ClientID: "c2", TopicFilter: "b/#"}}},
		ctx:            context.Background(),
		wakeClientFunc: func(string) {},
	}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:      uuid.New(),
		ShareGroup:  "g",
		TopicFilter: "a/#",
		MessageID:   uuid.New(),
		DeliveryQoS: 0,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}

	if err := c.processTask(task); err != nil {
		t.Fatalf("processTask error: %v", err)
	}
	if ts.calls != 1 || ts.lastPlan.ClientID != "c1" {
		t.Fatalf("expected task for only c1 on topic filter a/#, calls=%d plan=%+v", ts.calls, ts.lastPlan)
	}
}

func TestSharedConsumer_ProcessTask_AssignsSharedTaskWithoutCompletingIt(t *testing.T) {
	ts := &fakeTaskStore{}
	ss := &fakeSharedStore{}
	taskID := uuid.New()
	c := &SharedSubscriptionConsumer{
		shareGroup:     "g",
		topicFilter:    "a",
		store:          ss,
		taskStore:      ts,
		clientManager:  nil,
		selector:       staticSelector{clientID: "c1"},
		subCenter:      &fakeSubCenter{members: []*proto.ShareGroupMember{{ClientID: "c1", TopicFilter: "a"}}},
		ctx:            context.Background(),
		wakeClientFunc: func(string) {},
	}

	task := &sharedsubscription.ShareGroupTask{
		TaskID:      taskID,
		ShareGroup:  "g",
		TopicFilter: "a",
		MessageID:   uuid.New(),
		DeliveryQoS: 1,
		Status:      sharedsubscription.TaskStatusPending,
		Timestamp:   time.Now(),
	}

	if err := c.processTask(task); err != nil {
		t.Fatalf("processTask error: %v", err)
	}
	if ts.calls != 1 {
		t.Fatalf("expected AppendClientTask called once, got %d", ts.calls)
	}
	if ts.lastPlan.ShareGroup != "g" || ts.lastPlan.SharedTaskID != taskID {
		t.Fatalf("expected shared task metadata in client plan, got %+v", ts.lastPlan)
	}
	if ss.markCalls != 0 {
		t.Fatalf("shared task should not be marked completed before client delivery finishes, got %d calls", ss.markCalls)
	}
}

func assertConsumerHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string, want uint64) {
	t.Helper()
	if got := consumerHistogramSampleCount(t, collector, labels); got != want {
		t.Fatalf("histogram sample count for labels %v = %d, want %d", labels, got, want)
	}
}

func consumerHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string) uint64 {
	t.Helper()
	metrics := make(chan prometheus.Metric, 16)
	go func() {
		collector.Collect(metrics)
		close(metrics)
	}()
	for item := range metrics {
		dtoMetric := &dto.Metric{}
		if err := item.Write(dtoMetric); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		if !consumerMetricLabelsMatch(dtoMetric, labels) {
			continue
		}
		if dtoMetric.Histogram == nil {
			return 0
		}
		return dtoMetric.Histogram.GetSampleCount()
	}
	return 0
}

func consumerMetricLabelsMatch(item *dto.Metric, labels map[string]string) bool {
	if item == nil {
		return false
	}
	got := make(map[string]string, len(item.Label))
	for _, pair := range item.Label {
		got[pair.GetName()] = pair.GetValue()
	}
	for name, wantValue := range labels {
		if got[name] != wantValue {
			return false
		}
	}
	return true
}
