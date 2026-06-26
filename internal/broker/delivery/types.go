package delivery

import (
	"context"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
)

// Delivery pipeline abstractions.
// Goal: decouple routing (who should receive), persistence (delivery_task/message_body), and writer (how to send).
// This is the extension point for shared subscriptions (distribution policy) and alternative writers.

type ClientPlan struct {
	ClientID            string
	DeliveryQoS         int
	SubscriptionIDsJSON string
	// WinnerNoLocal/WinnerRAP are derived from the "winner" subscription among all matches for this client.
	WinnerNoLocal bool
	WinnerRAP     bool

	// Shared subscription metadata is set only for tasks assigned from a share group.
	ShareGroup   string
	SharedTaskID uuid.UUID
}

type RouteResult struct {
	Plans           []ClientPlan
	ShareGroupTasks []ShareGroupTask
}

// ShareGroupTask represents a task for shared subscription routing
type ShareGroupTask struct {
	ShareGroup      string
	TopicFilter     string
	DeliveryQoS     int
	PublishQoS      int
	PublisherClient string
	SubscriptionIDs string // JSON array string
	WinnerNoLocal   bool
	WinnerRAP       bool
}

type Router interface {
	Route(ctx context.Context, publish *packets.Publish, publisherClientID string) (*RouteResult, error)
}

type TaskStore interface {
	SavePublishMessage(ctx context.Context, ts time.Time, publisherClientID string, publish *packets.Publish, messageID uuid.UUID) (uuid.UUID, error)
	AppendClientTask(ctx context.Context, ts time.Time, clientID string, messageID uuid.UUID, plan ClientPlan) (taskID uuid.UUID, inserted bool, err error)
}

type CursorStore interface {
	ReadCursor(ctx context.Context, clientID string) (*store.DeliveryCursor, error)
	AdvanceCursor(ctx context.Context, cursor store.DeliveryCursor) error
	ReadTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error)
	LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error)
}

type ClientStateDeleter interface {
	DeleteClientState(ctx context.Context, clientID string) error
}

type SharedClientTaskDeleter interface {
	DeleteClientSharedTasks(ctx context.Context, clientID string, shareGroup string) error
}

// SchemaEnsurer is an optional interface for stores that can ensure required schema/tables.
// App can call this once at startup for fail-fast behavior.
type SchemaEnsurer interface {
	EnsureSchema(ctx context.Context) error
}
