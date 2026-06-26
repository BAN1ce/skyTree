package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/logger"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestAppendSharedDeliveryTasksReturnsAppendErrors(t *testing.T) {
	logger.LoadForTest()

	appendErr := errors.New("shared store unavailable")
	b := &Broker{
		shared: brokerSharedSubscriptionResources{
			store: &failingSharedSubscriptionStore{appendErr: appendErr},
		},
	}

	err := b.appendSharedDeliveryTasks(context.Background(), time.Now(), "topic/a", uuid.New(), []delivery.ShareGroupTask{
		{ShareGroup: "group-a", TopicFilter: "$share/group-a/topic/a", DeliveryQoS: 1},
	})
	if !errors.Is(err, appendErr) {
		t.Fatalf("expected shared append error, got %v", err)
	}
}

func TestAppendSharedDeliveryTasksSkipsExistingMessageID(t *testing.T) {
	store := &recordingSharedSubscriptionStore{
		existing: map[string]uuid.UUID{"group-a": uuid.New()},
	}
	messageID := store.existing["group-a"]
	b := &Broker{shared: brokerSharedSubscriptionResources{store: store}}

	err := b.appendSharedDeliveryTasks(context.Background(), time.Now(), "topic/a", messageID, []delivery.ShareGroupTask{
		{ShareGroup: "group-a", TopicFilter: "$share/group-a/topic/a", DeliveryQoS: 1},
		{ShareGroup: "group-b", TopicFilter: "$share/group-b/topic/a", DeliveryQoS: 1},
	})
	if err != nil {
		t.Fatalf("append shared delivery tasks: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected only one new share task, got %d", len(store.appended))
	}
	if store.appended[0].ShareGroup != "group-b" {
		t.Fatalf("expected group-b to be appended, got %q", store.appended[0].ShareGroup)
	}
}

func TestAppendSharedDeliveryTasksNotifiesManagerForNewTask(t *testing.T) {
	store := &recordingSharedSubscriptionStore{}
	notifier := &recordingSharedTaskNotifier{}
	b := &Broker{shared: brokerSharedSubscriptionResources{store: store, notifier: notifier}}
	messageID := uuid.New()

	err := b.appendSharedDeliveryTasks(context.Background(), time.Now(), "topic/a", messageID, []delivery.ShareGroupTask{
		{ShareGroup: "group-a", TopicFilter: "topic/a", DeliveryQoS: 1},
	})
	if err != nil {
		t.Fatalf("append shared delivery tasks: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected one appended task, got %d", len(store.appended))
	}
	if len(notifier.events) != 1 {
		t.Fatalf("expected one task appended notification, got %d", len(notifier.events))
	}
	event := notifier.events[0]
	if event.ShareGroup != "group-a" || event.TopicFilter != "topic/a" || event.TaskID != store.appended[0].TaskID {
		t.Fatalf("unexpected notification event: %+v appended=%+v", event, store.appended[0])
	}
}

func TestAppendSharedDeliveryTasksNotifiesManagerForExistingTask(t *testing.T) {
	messageID := uuid.New()
	taskID := uuid.New()
	store := &recordingSharedSubscriptionStore{
		existingTasks: map[string]*sharedsubscription.ShareGroupTask{
			"group-a": {
				TaskID:      taskID,
				ShareGroup:  "group-a",
				TopicFilter: "topic/a",
				MessageID:   messageID,
				Status:      sharedsubscription.TaskStatusPending,
			},
		},
	}
	notifier := &recordingSharedTaskNotifier{}
	b := &Broker{shared: brokerSharedSubscriptionResources{store: store, notifier: notifier}}

	err := b.appendSharedDeliveryTasks(context.Background(), time.Now(), "topic/a", messageID, []delivery.ShareGroupTask{
		{ShareGroup: "group-a", TopicFilter: "topic/a", DeliveryQoS: 1},
	})
	if err != nil {
		t.Fatalf("append shared delivery tasks: %v", err)
	}
	if len(store.appended) != 0 {
		t.Fatalf("expected existing task not to be appended, got %d", len(store.appended))
	}
	if len(notifier.events) != 1 {
		t.Fatalf("expected existing task notification, got %d", len(notifier.events))
	}
	event := notifier.events[0]
	if event.ShareGroup != "group-a" || event.TopicFilter != "topic/a" || event.TaskID != taskID {
		t.Fatalf("unexpected existing task event: %+v", event)
	}
}

func TestAppendClientDeliveryTasksSkipsExistingMessageID(t *testing.T) {
	messageID := uuid.New()
	store := &recordingTaskStore{
		existing: map[string]map[uuid.UUID]bool{
			"client-a": {messageID: true},
		},
	}
	b := &Broker{delivery: brokerDeliveryResources{taskStore: store}}

	err := b.appendClientDeliveryTasks(context.Background(), time.Now(), "topic/a", messageID, []delivery.ClientPlan{
		{ClientID: "client-a", DeliveryQoS: 1},
		{ClientID: "client-b", DeliveryQoS: 1},
	})
	if err != nil {
		t.Fatalf("append client delivery tasks: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected only one new client task, got %d", len(store.appended))
	}
	if store.appended[0].clientID != "client-b" {
		t.Fatalf("expected client-b to be appended, got %q", store.appended[0].clientID)
	}
}

func TestAppendClientDeliveryTasksRecordsEnqueueMetrics(t *testing.T) {
	messageID := uuid.New()
	appendErr := errors.New("append failed")
	store := &recordingTaskStore{
		existing: map[string]map[uuid.UUID]bool{
			"client-duplicate": {messageID: true},
		},
		appendErrByClient: map[string]error{
			"client-error": appendErr,
		},
	}
	b := &Broker{delivery: brokerDeliveryResources{taskStore: store}}

	successBefore := histogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "success",
	})
	duplicateBefore := histogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "duplicate",
	})
	errorBefore := histogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "error",
	})

	err := b.appendClientDeliveryTasks(context.Background(), time.Now(), "topic/a", messageID, []delivery.ClientPlan{
		{ClientID: "client-ok", DeliveryQoS: 1},
		{ClientID: "client-duplicate", DeliveryQoS: 1},
		{ClientID: "client-error", DeliveryQoS: 1},
	})
	if !errors.Is(err, appendErr) {
		t.Fatalf("expected append error, got %v", err)
	}

	assertHistogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "success",
	}, successBefore+1)
	assertHistogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "duplicate",
	}, duplicateBefore+1)
	assertHistogramSampleCount(t, metric.DeliveryEnqueueDelaySeconds, map[string]string{
		"path":   "normal",
		"qos":    "1",
		"result": "error",
	}, errorBefore+1)
}

func TestRoutePublishClientModeStoresRetainedPublishWithoutSubscribers(t *testing.T) {
	retainStore := retain.NewRetainStore(newCoreRetainMemKeyStore())
	b := &Broker{
		state: brokerStateCenters{
			retain: retainStore,
		},
		delivery: brokerDeliveryResources{
			taskStore: &recordingTaskStore{},
		},
	}
	publish := &packets.Publish{
		Topic:      "will/retain",
		QoS:        1,
		Retain:     true,
		Payload:    []byte("offline"),
		Properties: &packets.PublishProperties{},
	}

	err := b.routePublishClientMode(context.Background(), &brokerpublish.Message{
		SendClientID: "will-client",
		Publish:      publish,
		Will:         true,
	})
	if err != nil {
		t.Fatalf("handle internal retained publish: %v", err)
	}
	retained, ok := retainStore.GetRetainMessage("will/retain")
	if !ok {
		t.Fatal("expected retained publish to be stored")
	}
	if retained.GetPublisherClientID() != "will-client" {
		t.Fatalf("expected publisher will-client, got %q", retained.GetPublisherClientID())
	}
	if string(retained.GetPayload()) != "offline" {
		t.Fatalf("expected retained payload offline, got %q", retained.GetPayload())
	}
}

type failingSharedSubscriptionStore struct {
	appendErr error
}

func (s *failingSharedSubscriptionStore) EnsureSchema(context.Context) error {
	return nil
}

func (s *failingSharedSubscriptionStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	return s.appendErr
}

func (s *failingSharedSubscriptionStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

func (s *failingSharedSubscriptionStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	return nil
}

func (s *failingSharedSubscriptionStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	return false, nil
}

func (s *failingSharedSubscriptionStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

func (s *failingSharedSubscriptionStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	return nil
}

func (s *failingSharedSubscriptionStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

func (s *failingSharedSubscriptionStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}

func (s *failingSharedSubscriptionStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	return nil, nil
}

func (s *failingSharedSubscriptionStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	return nil
}

func (s *failingSharedSubscriptionStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

type recordingSharedSubscriptionStore struct {
	failingSharedSubscriptionStore
	existing      map[string]uuid.UUID
	existingTasks map[string]*sharedsubscription.ShareGroupTask
	appended      []*sharedsubscription.ShareGroupTask
}

func (s *recordingSharedSubscriptionStore) AppendShareGroupTask(_ context.Context, _ time.Time, task *sharedsubscription.ShareGroupTask) error {
	s.appended = append(s.appended, task)
	return nil
}

func (s *recordingSharedSubscriptionStore) QueryShareGroupTaskByMessageID(_ context.Context, shareGroup string, messageID uuid.UUID, statuses []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	if len(statuses) == 0 {
		return nil, errors.New("expected status filter")
	}
	if s.existingTasks != nil && s.existingTasks[shareGroup] != nil && s.existingTasks[shareGroup].MessageID == messageID {
		return s.existingTasks[shareGroup], nil
	}
	if s.existing != nil && s.existing[shareGroup] == messageID {
		return &sharedsubscription.ShareGroupTask{
			TaskID:     uuid.New(),
			ShareGroup: shareGroup,
			MessageID:  messageID,
			Status:     sharedsubscription.TaskStatusPending,
		}, nil
	}
	return nil, nil
}

type recordingSharedTaskNotifier struct {
	events []shared_manager.TaskAppendedEvent
}

func (n *recordingSharedTaskNotifier) NotifyTaskAppended(_ context.Context, event shared_manager.TaskAppendedEvent) error {
	n.events = append(n.events, event)
	return nil
}

type recordingTaskStore struct {
	existing          map[string]map[uuid.UUID]bool
	appendErrByClient map[string]error
	appended          []clientTaskAppend
}

type clientTaskAppend struct {
	clientID  string
	messageID uuid.UUID
	plan      delivery.ClientPlan
}

func (s *recordingTaskStore) SavePublishMessage(context.Context, time.Time, string, *packets.Publish, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (s *recordingTaskStore) AppendClientTask(_ context.Context, _ time.Time, clientID string, messageID uuid.UUID, plan delivery.ClientPlan) (uuid.UUID, bool, error) {
	if s.appendErrByClient != nil && s.appendErrByClient[clientID] != nil {
		return uuid.Nil, false, s.appendErrByClient[clientID]
	}
	if s.existing != nil && s.existing[clientID] != nil && s.existing[clientID][messageID] {
		return uuid.New(), false, nil
	}
	s.appended = append(s.appended, clientTaskAppend{clientID: clientID, messageID: messageID, plan: plan})
	return uuid.New(), true, nil
}

func (s *recordingTaskStore) ClientTaskExists(_ context.Context, clientID string, messageID uuid.UUID) (bool, error) {
	if s.existing == nil || s.existing[clientID] == nil {
		return false, nil
	}
	return s.existing[clientID][messageID], nil
}

var _ delivery.TaskStore = (*recordingTaskStore)(nil)

type coreRetainMemKeyStore struct {
	mu     sync.Mutex
	hashes map[string]map[string][]byte
}

func newCoreRetainMemKeyStore() *coreRetainMemKeyStore {
	return &coreRetainMemKeyStore{
		hashes: make(map[string]map[string][]byte),
	}
}

func (m *coreRetainMemKeyStore) PutKey(context.Context, []byte, []byte) error { return nil }

func (m *coreRetainMemKeyStore) ReadKey(context.Context, []byte) ([]byte, bool, error) {
	return nil, false, nil
}

func (m *coreRetainMemKeyStore) DeleteKey(context.Context, []byte) error { return nil }

func (m *coreRetainMemKeyStore) DeletePrefixKey(context.Context, []byte) error { return nil }

func (m *coreRetainMemKeyStore) HSet(_ context.Context, key []byte, field [][]byte) error {
	if len(field) < 2 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	k := string(key)
	h, ok := m.hashes[k]
	if !ok {
		h = make(map[string][]byte)
		m.hashes[k] = h
	}
	h[string(field[0])] = append([]byte(nil), field[1]...)
	return nil
}

func (m *coreRetainMemKeyStore) HGet(_ context.Context, key, field []byte) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[string(key)]
	if !ok {
		return nil, false, nil
	}
	value, ok := h[string(field)]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (m *coreRetainMemKeyStore) HDel(_ context.Context, key []byte, field [][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[string(key)]
	if !ok {
		return nil
	}
	for _, item := range field {
		delete(h, string(item))
	}
	return nil
}

func (m *coreRetainMemKeyStore) HGetAll(_ context.Context, key []byte) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string)
	for field, value := range m.hashes[string(key)] {
		out[field] = string(value)
	}
	return out, nil
}

func (m *coreRetainMemKeyStore) HPrefix(context.Context, []byte, []byte) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *coreRetainMemKeyStore) DeleteHash(_ context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hashes, string(key))
	return nil
}

func (m *coreRetainMemKeyStore) Close() error { return nil }

func (m *coreRetainMemKeyStore) SetExpired(context.Context, []byte, time.Duration) error {
	return nil
}

var _ brokerstore.HashStore = (*coreRetainMemKeyStore)(nil)

func assertHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string, want uint64) {
	t.Helper()
	if got := histogramSampleCount(t, collector, labels); got != want {
		t.Fatalf("histogram sample count for labels %v = %d, want %d", labels, got, want)
	}
}

func histogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string) uint64 {
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
		if !metricLabelsMatch(dtoMetric, labels) {
			continue
		}
		if dtoMetric.Histogram == nil {
			return 0
		}
		return dtoMetric.Histogram.GetSampleCount()
	}
	return 0
}

func metricLabelsMatch(item *dto.Metric, labels map[string]string) bool {
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
