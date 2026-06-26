package delivery

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/google/uuid"
)

type ClientDeliveryCursorStore struct {
	persistence *DeliveryPersistence
}

func NewClientDeliveryCursorStore(clientDeliveryStore store.ClientDeliveryStore) (*ClientDeliveryCursorStore, error) {
	persistence, err := NewDeliveryPersistence(clientDeliveryStore, serializer.Serializer)
	if err != nil {
		return nil, err
	}
	return &ClientDeliveryCursorStore{persistence: persistence}, nil
}

func (s *ClientDeliveryCursorStore) EnsureSchema(ctx context.Context) error {
	return s.persistence.EnsureSchema(ctx)
}

func (s *ClientDeliveryCursorStore) ReadCursor(ctx context.Context, clientID string) (*store.DeliveryCursor, error) {
	return s.persistence.ReadDeliveryCursor(ctx, clientID)
}

func (s *ClientDeliveryCursorStore) AdvanceCursor(ctx context.Context, cursor store.DeliveryCursor) error {
	return s.persistence.AdvanceDeliveryCursor(ctx, cursor)
}

func (s *ClientDeliveryCursorStore) ReadTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
	return s.persistence.ReadDeliveryTasks(ctx, clientID, lastTS, lastTaskID, limit)
}

func (s *ClientDeliveryCursorStore) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	return s.persistence.LoadMessagePayload(ctx, messageID)
}

func (s *ClientDeliveryCursorStore) DeleteClientState(ctx context.Context, clientID string) error {
	return s.persistence.DeleteClientDeliveryState(ctx, clientID)
}

func (s *ClientDeliveryCursorStore) DeleteClientSharedTasks(ctx context.Context, clientID string, shareGroup string) error {
	return s.persistence.DeleteClientSharedTasks(ctx, clientID, shareGroup)
}
