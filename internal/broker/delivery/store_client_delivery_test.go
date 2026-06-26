package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
)

func TestSavePublishMessageUsesProvidedMessageID(t *testing.T) {
	taskStore, err := NewClientDeliveryTaskStore(&recordingClientDeliveryStore{})
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	publish := &packets.Publish{
		Topic:      "delivery/retry",
		QoS:        1,
		PacketID:   7,
		Payload:    []byte("payload"),
		Properties: &packets.PublishProperties{},
	}

	providedID := uuid.New()
	id1, err := taskStore.SavePublishMessage(context.Background(), time.Unix(1, 0), "publisher-a", cloneTestPublish(publish), providedID)
	if err != nil {
		t.Fatalf("save first publish: %v", err)
	}
	id2, err := taskStore.SavePublishMessage(context.Background(), time.Unix(2, 0), "publisher-a", cloneTestPublish(publish), providedID)
	if err != nil {
		t.Fatalf("save retry publish: %v", err)
	}
	if id1 != providedID || id2 != providedID {
		t.Fatalf("expected provided message ID to be used, got %s and %s", id1, id2)
	}
}

func TestSavePublishMessageGeneratesIDWhenNotProvided(t *testing.T) {
	taskStore, err := NewClientDeliveryTaskStore(&recordingClientDeliveryStore{})
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	publish := &packets.Publish{
		Topic:      "delivery/retry",
		QoS:        1,
		PacketID:   7,
		Payload:    []byte("payload"),
		Properties: &packets.PublishProperties{},
	}

	id1, err := taskStore.SavePublishMessage(context.Background(), time.Unix(1, 0), "publisher-a", cloneTestPublish(publish), uuid.Nil)
	if err != nil {
		t.Fatalf("save first publish: %v", err)
	}
	id2, err := taskStore.SavePublishMessage(context.Background(), time.Unix(2, 0), "publisher-a", cloneTestPublish(publish), uuid.Nil)
	if err != nil {
		t.Fatalf("save second publish: %v", err)
	}
	if id1 == uuid.Nil || id2 == uuid.Nil {
		t.Fatalf("expected generated message IDs, got %s and %s", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("expected different generated IDs, got %s", id1)
	}
}

func TestAppendClientTaskReportsWhetherTaskWasInserted(t *testing.T) {
	recording := &recordingClientDeliveryStore{appendInserted: false}
	taskStore, err := NewClientDeliveryTaskStore(recording)
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}

	taskID, inserted, err := taskStore.AppendClientTask(
		context.Background(),
		time.Unix(3, 0),
		"client-a",
		uuid.MustParse("00000000-0000-0000-0000-000000000701"),
		ClientPlan{DeliveryQoS: 1},
	)
	if err != nil {
		t.Fatalf("AppendClientTask error: %v", err)
	}
	if taskID == uuid.Nil {
		t.Fatal("expected generated taskID")
	}
	if inserted {
		t.Fatal("expected duplicate append to be reported as not inserted")
	}
}

func TestSavePublishMessageTreatsZeroMessageExpiryAsImmediateExpiry(t *testing.T) {
	recording := &recordingClientDeliveryStore{}
	taskStore, err := NewClientDeliveryTaskStore(recording)
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	expiry := uint32(0)
	publish := &packets.Publish{
		Topic:    "delivery/expiry",
		QoS:      1,
		PacketID: 1,
		Payload:  []byte("payload"),
		Properties: &packets.PublishProperties{
			MessageExpiry: &expiry,
		},
	}
	ts := time.Unix(10, 123)

	if _, err := taskStore.SavePublishMessage(context.Background(), ts, "publisher-a", cloneTestPublish(publish), uuid.Nil); err != nil {
		t.Fatalf("save publish: %v", err)
	}
	if len(recording.payloads) != 1 {
		t.Fatalf("expected one payload record, got %d", len(recording.payloads))
	}
	decoded, err := serializer.Serializer.Decode(recording.payloads[0].Payload)
	if err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	if decoded.ExpiredTime != ts.UnixNano() {
		t.Fatalf("expected immediate expiry at created time %d, got %d", ts.UnixNano(), decoded.ExpiredTime)
	}
}

func TestSavePublishMessageAllowsInternalQoS1PublishWithoutPacketID(t *testing.T) {
	recording := &recordingClientDeliveryStore{}
	taskStore, err := NewClientDeliveryTaskStore(recording)
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	publish := &packets.Publish{
		Topic:      "delivery/will",
		QoS:        1,
		PacketID:   0,
		Payload:    []byte("offline"),
		Properties: &packets.PublishProperties{},
	}

	if _, err := taskStore.SavePublishMessage(context.Background(), time.Unix(11, 0), "publisher-a", publish, uuid.Nil); err != nil {
		t.Fatalf("save internal publish: %v", err)
	}
	if len(recording.payloads) != 1 {
		t.Fatalf("expected one payload record, got %d", len(recording.payloads))
	}
}

func TestSavePublishMessageClearsStoredPacketIDWithoutMutatingCaller(t *testing.T) {
	recording := &recordingClientDeliveryStore{}
	taskStore, err := NewClientDeliveryTaskStore(recording)
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	publish := &packets.Publish{
		Topic:      "delivery/publish",
		QoS:        1,
		PacketID:   7,
		Payload:    []byte("payload"),
		Properties: &packets.PublishProperties{},
	}

	if _, err := taskStore.SavePublishMessage(context.Background(), time.Unix(12, 0), "publisher-a", publish, uuid.Nil); err != nil {
		t.Fatalf("save publish: %v", err)
	}
	if publish.PacketID != 7 {
		t.Fatalf("caller publish PacketID mutated to %d", publish.PacketID)
	}
	decoded, err := serializer.Serializer.Decode(recording.payloads[0].Payload)
	if err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	storedPublish := decoded.GetPublish()
	if storedPublish == nil {
		t.Fatal("expected stored publish")
	}
	if storedPublish.PacketID != 0 {
		t.Fatalf("expected stored publish PacketID=0, got %d", storedPublish.PacketID)
	}
}

func cloneTestPublish(p *packets.Publish) *packets.Publish {
	cp := *p
	cp.Payload = append([]byte(nil), p.Payload...)
	if p.Properties != nil {
		props := *p.Properties
		cp.Properties = &props
	}
	return &cp
}

type recordingClientDeliveryStore struct {
	payloads       []store.MessagePayloadRecord
	tasks          []store.DeliveryTask
	appendInserted bool
}

func (s *recordingClientDeliveryStore) EnsureDeliverySchema(context.Context) error {
	return nil
}

func (s *recordingClientDeliveryStore) SaveMessagePayload(_ context.Context, record store.MessagePayloadRecord) error {
	s.payloads = append(s.payloads, record)
	return nil
}

func (s *recordingClientDeliveryStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, store.ErrMessagePayloadNotFound
}

func (s *recordingClientDeliveryStore) AppendDeliveryTask(_ context.Context, task store.DeliveryTask) (bool, error) {
	s.tasks = append(s.tasks, task)
	return s.appendInserted, nil
}

func (s *recordingClientDeliveryStore) ReadDeliveryTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*store.DeliveryTask, error) {
	return nil, nil
}

func (s *recordingClientDeliveryStore) AdvanceDeliveryCursor(context.Context, store.DeliveryCursor) (bool, error) {
	return true, nil
}

func (s *recordingClientDeliveryStore) ReadDeliveryCursor(context.Context, string) (*store.DeliveryCursor, error) {
	return nil, nil
}

func (s *recordingClientDeliveryStore) ResetClientDeliveryState(context.Context, string) error {
	return nil
}

func (s *recordingClientDeliveryStore) DeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}

var _ store.ClientDeliveryStore = (*recordingClientDeliveryStore)(nil)
