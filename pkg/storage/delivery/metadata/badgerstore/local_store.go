package badgerstore

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/fsutil"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
)

// LocalDeliveryQueueStore is a non-Raft implementation backed by a local Badger DB.
// It is used when cluster mode is disabled.
type LocalDeliveryQueueStore struct {
	sm *DeliveryStateMachine
}

var _ store.DeliveryQueueStore = (*LocalDeliveryQueueStore)(nil)
var _ store.DeliveryBacklogSummaryStore = (*LocalDeliveryQueueStore)(nil)

func NewLocalDeliveryQueueStore(basePath string, nodeID uint64) (*LocalDeliveryQueueStore, error) {
	path := filepath.Join(basePath, fmt.Sprintf("%d", nodeID), "delivery_queue_local")
	if err := fsutil.CreateDir(path); err != nil {
		return nil, err
	}
	opt := badger.DefaultOptions(path)
	opt.SyncWrites = false
	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}
	return &LocalDeliveryQueueStore{sm: NewDeliveryStateMachine(db)}, nil
}

func (s *LocalDeliveryQueueStore) EnsureDeliverySchema(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := marshalUpdate(opEnsureSchema, nil)
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) AppendDeliveryTask(ctx context.Context, task store.DeliveryTask) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	sharedTaskID := ""
	if task.SharedTaskID != uuid.Nil {
		sharedTaskID = task.SharedTaskID.String()
	}
	b, err := marshalUpdate(opAppendTask, appendTaskRequest{
		TSUnixNano:        task.TS.UnixNano(),
		TaskID:            task.TaskID.String(),
		ClientID:          task.ClientID,
		MessageID:         task.MessageID.String(),
		DeliveryQoS:       task.DeliveryQoS,
		SubscriptionIDs:   task.SubscriptionIDs,
		NoLocal:           task.NoLocal,
		RetainAsPublished: task.RetainAsPublished,
		ShareGroup:        task.ShareGroup,
		SharedTaskID:      sharedTaskID,
	})
	if err != nil {
		return false, err
	}
	result, err := s.sm.Update(b)
	return result.Value == 1, err
}

func (s *LocalDeliveryQueueStore) ReadDeliveryTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&readTasksQuery{
		ClientID:       clientID,
		LastTSUnixNano: lastTS.UnixNano(),
		LastTaskID:     lastTaskID,
		Limit:          limit,
	})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.([]*store.DeliveryTask); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) AdvanceDeliveryCursor(ctx context.Context, cursor store.DeliveryCursor) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	b, err := marshalUpdate(opAdvanceCursor, advanceCursorRequest{
		UpdatedTSUnixNano: cursor.UpdatedTS.UnixNano(),
		ClientID:          cursor.ClientID,
		LastTSUnixNano:    cursor.LastTS.UnixNano(),
		LastTaskID:        cursor.LastTaskID.String(),
	})
	if err != nil {
		return false, err
	}
	result, err := s.sm.Update(b)
	return result.Value == 1, err
}

func (s *LocalDeliveryQueueStore) ReadDeliveryCursor(ctx context.Context, clientID string) (*store.DeliveryCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := s.sm.Lookup(&readCursorQuery{ClientID: clientID})
	if err != nil || resp == nil {
		return nil, err
	}
	if out, ok := resp.(*store.DeliveryCursor); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) DeliveryBacklogSummary(ctx context.Context) (store.DeliveryBacklogSummary, error) {
	if err := ctx.Err(); err != nil {
		return store.DeliveryBacklogSummary{}, err
	}
	resp, err := s.sm.Lookup(&readBacklogSummaryQuery{})
	if err != nil || resp == nil {
		return store.DeliveryBacklogSummary{}, err
	}
	if out, ok := resp.(*store.DeliveryBacklogSummary); ok {
		return *out, nil
	}
	return store.DeliveryBacklogSummary{}, fmt.Errorf("unexpected response type %T", resp)
}

func (s *LocalDeliveryQueueStore) ResetClientDeliveryState(ctx context.Context, clientID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if clientID == "" {
		return fmt.Errorf("clientID is empty")
	}
	b, err := marshalUpdate(opDeleteClientState, deleteClientStateRequest{ClientID: clientID})
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) DeliveryTaskExists(
	ctx context.Context,
	clientID string,
	messageID uuid.UUID,
) (bool, error) {
	return checkDeliveryTaskExists(ctx, s, clientID, messageID)
}

func (s *LocalDeliveryQueueStore) DeleteClientSharedDeliveryTasks(ctx context.Context, clientID string, shareGroup string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if clientID == "" {
		return fmt.Errorf("clientID is empty")
	}
	if shareGroup == "" {
		return fmt.Errorf("shareGroup is empty")
	}
	b, err := marshalUpdate(opDeleteSharedTasks, deleteSharedTasksRequest{ClientID: clientID, ShareGroup: shareGroup})
	if err != nil {
		return err
	}
	_, err = s.sm.Update(b)
	return err
}

func (s *LocalDeliveryQueueStore) Close() error {
	if s == nil || s.sm == nil {
		return nil
	}
	return s.sm.Close()
}
