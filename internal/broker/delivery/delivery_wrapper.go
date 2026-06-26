package delivery

import (
	"context"
	"fmt"
	"time"

	broker_store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/google/uuid"
)

const defaultDeliveryStoreTimeout = 10 * time.Second

type DeliveryPersistence struct {
	store      broker_store.ClientDeliveryStore
	serializer broker_store.Serializer
	timeout    time.Duration
}

type DeliveryPersistenceOption func(*DeliveryPersistence)

func WithDeliveryPersistenceTimeout(timeout time.Duration) DeliveryPersistenceOption {
	return func(p *DeliveryPersistence) {
		if timeout > 0 {
			p.timeout = timeout
		}
	}
}

func NewDeliveryPersistence(clientDeliveryStore broker_store.ClientDeliveryStore, serializer broker_store.Serializer, opts ...DeliveryPersistenceOption) (*DeliveryPersistence, error) {
	if clientDeliveryStore == nil {
		return nil, fmt.Errorf("clientDeliveryStore is nil")
	}
	if serializer == nil {
		return nil, fmt.Errorf("serializer is nil")
	}
	p := &DeliveryPersistence{
		store:      clientDeliveryStore,
		serializer: serializer,
		timeout:    defaultDeliveryStoreTimeout,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *DeliveryPersistence) EnsureSchema(ctx context.Context) error {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.EnsureDeliverySchema(innerCtx)
}

// SaveMessagePayload serializes and stores the message once.
// If message.MessageID is empty, a new UUIDv7 is generated.
func (p *DeliveryPersistence) SaveMessagePayload(ctx context.Context, ts time.Time, publishTopic string, publisherClientID string, message *brokerpublish.Message) (uuid.UUID, error) {
	if message == nil {
		return uuid.Nil, fmt.Errorf("message is nil")
	}
	if message.MessageID == uuid.Nil {
		uid, err := uuid.NewV7()
		if err != nil {
			return uuid.Nil, err
		}
		message.MessageID = uid
	}
	if len(message.EncodeData) == 0 {
		raw, err := p.serializer.Encode(message)
		if err != nil {
			return uuid.Nil, err
		}
		message.EncodeData = raw
	}
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if err := p.store.SaveMessagePayload(innerCtx, broker_store.MessagePayloadRecord{
		CreatedAt:         ts,
		MessageID:         message.MessageID,
		PublishTopic:      publishTopic,
		PublisherClientID: publisherClientID,
		Payload:           message.EncodeData,
	}); err != nil {
		return uuid.Nil, err
	}
	return message.MessageID, nil
}

// AppendDeliveryTask appends one delivery task.
func (p *DeliveryPersistence) AppendDeliveryTask(ctx context.Context, task broker_store.DeliveryTask) (uuid.UUID, bool, error) {
	if task.TaskID == uuid.Nil {
		taskID, err := uuid.NewV7()
		if err != nil {
			return uuid.Nil, false, err
		}
		task.TaskID = taskID
	}
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	inserted, err := p.store.AppendDeliveryTask(innerCtx, task)
	if err != nil {
		return uuid.Nil, false, err
	}
	return task.TaskID, inserted, nil
}

func (p *DeliveryPersistence) ReadDeliveryCursor(ctx context.Context, clientID string) (*broker_store.DeliveryCursor, error) {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.ReadDeliveryCursor(innerCtx, clientID)
}

func (p *DeliveryPersistence) AdvanceDeliveryCursor(ctx context.Context, cursor broker_store.DeliveryCursor) error {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	_, err := p.store.AdvanceDeliveryCursor(innerCtx, cursor)
	return err
}

func (p *DeliveryPersistence) ReadDeliveryTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*broker_store.DeliveryTask, error) {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.ReadDeliveryTasks(innerCtx, clientID, lastTS, lastTaskID, limit)
}

func (p *DeliveryPersistence) DeliveryTaskExists(ctx context.Context, clientID string, messageID uuid.UUID) (bool, error) {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.DeliveryTaskExists(innerCtx, clientID, messageID)
}

func (p *DeliveryPersistence) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.LoadMessagePayload(innerCtx, messageID)
}

func (p *DeliveryPersistence) DeleteClientDeliveryState(ctx context.Context, clientID string) error {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.store.ResetClientDeliveryState(innerCtx, clientID)
}

func (p *DeliveryPersistence) DeleteClientSharedTasks(ctx context.Context, clientID string, shareGroup string) error {
	innerCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	deleter, ok := p.store.(interface {
		DeleteClientSharedDeliveryTasks(context.Context, string, string) error
	})
	if !ok {
		return nil
	}
	return deleter.DeleteClientSharedDeliveryTasks(innerCtx, clientID, shareGroup)
}
