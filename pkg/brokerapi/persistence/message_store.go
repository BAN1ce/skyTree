package persistence

import (
	"context"
	"errors"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/google/uuid"
)

var ErrMessagePayloadNotFound = errors.Join(ErrNotFound, errors.New("message payload not found"))

// MessagePayloadRecord is the serialized MQTT message stored once per publish.
type MessagePayloadRecord struct {
	CreatedAt         time.Time
	MessageID         uuid.UUID
	PublishTopic      string
	PublisherClientID string
	Payload           []byte
}

// MessagePayloadStore stores and loads serialized message payload bytes by messageID.
type MessagePayloadStore interface {
	SaveMessagePayload(ctx context.Context, record MessagePayloadRecord) error
	LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error)
}

// DeliveryQueueStore stores and reads client delivery tasks and cursors.
// It is the metadata plane of the client-centric delivery pipeline.
type DeliveryQueueStore interface {
	// EnsureDeliverySchema creates required tables/structures if missing.
	EnsureDeliverySchema(ctx context.Context) error

	// AppendDeliveryTask appends a delivery task for one client.
	// It returns inserted=false when an equivalent client/message task already exists.
	AppendDeliveryTask(ctx context.Context, task DeliveryTask) (inserted bool, err error)

	// DeliveryTaskExists checks whether a client already has a task for the message.
	DeliveryTaskExists(ctx context.Context, clientID string, messageID uuid.UUID) (bool, error)

	// ReadDeliveryTasks reads tasks after the given cursor (exclusive), ordered by (ts, task_id).
	ReadDeliveryTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*DeliveryTask, error)

	// AdvanceDeliveryCursor advances cursor for a client and allows implementations to clean up acknowledged tasks.
	// It returns advanced=false when the requested cursor does not move the stored cursor forward.
	AdvanceDeliveryCursor(ctx context.Context, cursor DeliveryCursor) (advanced bool, err error)
	ReadDeliveryCursor(ctx context.Context, clientID string) (*DeliveryCursor, error)

	// ResetClientDeliveryState clears or logically resets delivery tasks and cursor state owned by a client.
	ResetClientDeliveryState(ctx context.Context, clientID string) error
}

// DeliveryBacklogSummary aggregates queue backlog size and active-client footprint.
type DeliveryBacklogSummary struct {
	PendingTasks  int64
	ActiveClients int64
}

// DeliveryBacklogSummaryStore is an optional capability for queue-level summary stats.
type DeliveryBacklogSummaryStore interface {
	DeliveryBacklogSummary(ctx context.Context) (DeliveryBacklogSummary, error)
}

// ClientDeliveryStore is the combined store interface for the client-centric delivery pipeline.
// It composes the payload store and the delivery queue metadata store.
type ClientDeliveryStore interface {
	MessagePayloadStore
	DeliveryQueueStore
}

// DeliveryTask is a generic row representation returned by ClientDeliveryStore.
type DeliveryTask struct {
	// TS is the task enqueue timestamp and, together with TaskID, forms a stable ordered cursor.
	TS time.Time
	// TaskID is the unique identity of this delivery-task row (task plane identity).
	// Multiple tasks can point to the same MessageID when one publish fans out to multiple clients.
	TaskID uuid.UUID
	// ClientID is the target receiver client of this task.
	ClientID string
	// MessageID is the logical identity of the publish payload (message plane identity).
	// It is used for payload lookup and per-client dedupe.
	MessageID uuid.UUID
	// Generation is the client delivery-state generation used by cursor advance and state reset semantics.
	Generation int64
	// DeliveryQoS is the effective QoS selected for this client delivery.
	DeliveryQoS int
	// SubscriptionIDs are MQTT5 subscription identifiers matched for this client.
	SubscriptionIDs []int32
	// NoLocal/RetainAsPublished are derived from the winning subscription among all matches for this client.
	NoLocal           bool
	RetainAsPublished bool

	// Shared subscription metadata is present only when the task came from a shared group.
	ShareGroup   string
	SharedTaskID uuid.UUID
}

// DeliveryCursor is a generic cursor state returned by ClientDeliveryStore.
type DeliveryCursor struct {
	UpdatedTS  time.Time
	ClientID   string
	Generation int64
	LastTS     time.Time
	LastTaskID uuid.UUID
}

type Serializer interface {
	Encode(publish *brokerpublish.Message) (data []byte, err error)
	Decode(rawData []byte) (*brokerpublish.Message, error)
}
