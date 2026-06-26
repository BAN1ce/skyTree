package badgerstore

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
	"github.com/lni/dragonboat/v3/statemachine"
)

func decodeUpdateData[T any](req *updateRequest) (T, error) {
	var r T
	err := json.Unmarshal(req.Data, &r)
	return r, err
}

func (s *DeliveryStateMachine) updateAppendTask(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[appendTaskRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ClientID == "" {
		return statemachine.Result{}, fmt.Errorf("client_id is empty")
	}
	taskUUID, err := uuid.Parse(r.TaskID)
	if err != nil {
		return statemachine.Result{}, err
	}
	taskBytes := [16]byte(taskUUID)
	val, err := json.Marshal(taskValue{
		MessageID:         r.MessageID,
		DeliveryQoS:       r.DeliveryQoS,
		SubscriptionIDs:   cloneSubscriptionIDs(r.SubscriptionIDs),
		NoLocal:           r.NoLocal,
		RetainAsPublished: r.RetainAsPublished,
		ShareGroup:        r.ShareGroup,
		SharedTaskID:      r.SharedTaskID,
	})
	if err != nil {
		return statemachine.Result{}, err
	}
	key := taskKey(r.ClientID, r.TSUnixNano, taskBytes)
	var inserted bool
	err = withBadgerUpdateRetry(func() error {
		inserted = false
		return s.db.Update(func(txn *badger.Txn) error {
			exists, err := deliveryTaskExistsInTxn(txn, r.ClientID, r.MessageID)
			if err != nil || exists {
				return err
			}
			inserted = true
			return txn.Set(key, val)
		})
	})
	if inserted {
		return statemachine.Result{Value: 1}, err
	}
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateAdvanceCursor(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[advanceCursorRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ClientID == "" {
		return statemachine.Result{}, fmt.Errorf("client_id is empty")
	}
	if r.LastTaskID == "" {
		return statemachine.Result{}, fmt.Errorf("last_task_id is empty")
	}
	val, err := json.Marshal(cursorValue{
		UpdatedTSUnixNano: r.UpdatedTSUnixNano,
		ClientID:          r.ClientID,
		LastTSUnixNano:    r.LastTSUnixNano,
		LastTaskID:        r.LastTaskID,
	})
	if err != nil {
		return statemachine.Result{}, err
	}
	key := cursorKey(r.ClientID)
	var advanced bool
	err = s.db.Update(func(txn *badger.Txn) error {
		current, err := lookupDeliveryCursorInTxn(txn, r.ClientID)
		if err != nil {
			return err
		}
		nextTaskID, err := uuid.Parse(r.LastTaskID)
		if err != nil {
			return err
		}
		if !deliveryPositionAfter(r.LastTSUnixNano, nextTaskID, current) {
			return nil
		}
		if err := txn.Set(key, val); err != nil {
			return err
		}
		advanced = true
		return deleteDeliveryTasksThroughCursor(txn, r.ClientID, r.LastTSUnixNano, r.LastTaskID)
	})
	if advanced {
		return statemachine.Result{Value: 1}, err
	}
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateDeleteClientState(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[deleteClientStateRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ClientID == "" {
		return statemachine.Result{}, fmt.Errorf("client_id is empty")
	}
	err = s.db.Update(func(txn *badger.Txn) error {
		return deleteClientDeliveryState(txn, r.ClientID)
	})
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateDeleteSharedTasks(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[deleteSharedTasksRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ClientID == "" {
		return statemachine.Result{}, fmt.Errorf("client_id is empty")
	}
	if r.ShareGroup == "" {
		return statemachine.Result{}, fmt.Errorf("share_group is empty")
	}
	err = s.db.Update(func(txn *badger.Txn) error {
		return deleteClientSharedDeliveryTasks(txn, r.ClientID, r.ShareGroup)
	})
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateAppendShareTask(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[appendShareTaskRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ShareGroup == "" {
		return statemachine.Result{}, fmt.Errorf("share_group is empty")
	}
	taskUUID, err := uuid.Parse(r.TaskID)
	if err != nil {
		return statemachine.Result{}, err
	}
	status := r.Status
	if status == "" {
		status = sharedsubscription.TaskStatusPending
	}
	val, err := json.Marshal(shareTaskValue{
		ShareGroup:      r.ShareGroup,
		TopicFilter:     r.TopicFilter,
		MessageID:       r.MessageID,
		DeliveryQoS:     r.DeliveryQoS,
		PublishQoS:      r.PublishQoS,
		PublisherClient: r.PublisherClient,
		SubscriptionIDs: r.SubscriptionIDs,
		WinnerNoLocal:   r.WinnerNoLocal,
		WinnerRAP:       r.WinnerRAP,
		Status:          status,
		ProcessedByNode: r.ProcessedByNode,
		ProcessedAtNano: r.ProcessedAtNano,
		RollbackReason:  r.RollbackReason,
	})
	if err != nil {
		return statemachine.Result{}, err
	}
	taskBytes := [16]byte(taskUUID)
	key := shareTaskKey(r.ShareGroup, r.TSUnixNano, taskBytes)
	return statemachine.Result{}, s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *DeliveryStateMachine) updateShareTaskStatus(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[updateShareTaskStatusRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ShareGroup == "" {
		return statemachine.Result{}, fmt.Errorf("share_group is empty")
	}
	taskUUID, err := uuid.Parse(r.TaskID)
	if err != nil {
		return statemachine.Result{}, err
	}
	var updated bool
	err = s.db.Update(func(txn *badger.Txn) error {
		updated, err = updateShareTaskStatus(txn, r.ShareGroup, taskUUID, r.OldStatus, r.NewStatus, r.NowNano)
		return err
	})
	if updated {
		return statemachine.Result{Value: 1}, err
	}
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateRollbackShareGroupTask(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[appendShareTaskRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ShareGroup == "" {
		return statemachine.Result{}, fmt.Errorf("share_group is empty")
	}
	err = s.db.Update(func(txn *badger.Txn) error {
		return rollbackShareTask(txn, r)
	})
	return statemachine.Result{}, err
}

func (s *DeliveryStateMachine) updateAppendShareGroupCursor(req *updateRequest) (statemachine.Result, error) {
	r, err := decodeUpdateData[appendShareGroupCursorRequest](req)
	if err != nil {
		return statemachine.Result{}, err
	}
	if r.ShareGroup == "" {
		return statemachine.Result{}, fmt.Errorf("share_group is empty")
	}
	val, err := json.Marshal(shareCursorValue{
		ShareGroup:          r.ShareGroup,
		LastProcessedTSNano: r.LastProcessedTSNano,
		LastProcessedTaskID: r.LastProcessedTaskID,
		LeaderNodeID:        r.LeaderNodeID,
		LastRenewalNano:     r.LastRenewalNano,
	})
	if err != nil {
		return statemachine.Result{}, err
	}
	key := shareCursorKey(r.ShareGroup)
	return statemachine.Result{}, s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func deleteDeliveryTasksThroughCursor(txn *badger.Txn, clientID string, lastTSUnixNano int64, lastTaskID string) error {
	taskUUID, err := uuid.Parse(lastTaskID)
	if err != nil {
		return err
	}
	lastTaskBytes := [16]byte(taskUUID)
	endKey := taskKey(clientID, lastTSUnixNano, lastTaskBytes)
	clientPrefix := taskKeyPrefix(clientID)

	itOpt := badger.DefaultIteratorOptions
	itOpt.PrefetchValues = false
	it := txn.NewIterator(itOpt)
	defer it.Close()

	for it.Seek(clientPrefix); it.ValidForPrefix(clientPrefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		if bytes.Compare(key, endKey) > 0 {
			break
		}
		if err := txn.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func deleteClientDeliveryState(txn *badger.Txn, clientID string) error {
	clientPrefix := taskKeyPrefix(clientID)

	itOpt := badger.DefaultIteratorOptions
	itOpt.PrefetchValues = false
	it := txn.NewIterator(itOpt)
	defer it.Close()

	for it.Seek(clientPrefix); it.ValidForPrefix(clientPrefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		if err := txn.Delete(key); err != nil {
			return err
		}
	}
	if err := txn.Delete(cursorKey(clientID)); err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	return nil
}

func deliveryTaskExistsInTxn(txn *badger.Txn, clientID string, messageID string) (bool, error) {
	if clientID == "" || messageID == "" {
		return false, nil
	}
	clientPrefix := taskKeyPrefix(clientID)

	itOpt := badger.DefaultIteratorOptions
	itOpt.PrefetchValues = true
	it := txn.NewIterator(itOpt)
	defer it.Close()

	for it.Seek(clientPrefix); it.ValidForPrefix(clientPrefix); it.Next() {
		var tv taskValue
		if err := it.Item().Value(func(val []byte) error {
			return json.Unmarshal(val, &tv)
		}); err != nil {
			return false, err
		}
		if tv.MessageID == messageID {
			return true, nil
		}
	}
	return false, nil
}

func deleteClientSharedDeliveryTasks(txn *badger.Txn, clientID string, shareGroup string) error {
	clientPrefix := taskKeyPrefix(clientID)

	itOpt := badger.DefaultIteratorOptions
	itOpt.PrefetchValues = true
	it := txn.NewIterator(itOpt)
	defer it.Close()

	for it.Seek(clientPrefix); it.ValidForPrefix(clientPrefix); it.Next() {
		item := it.Item()
		shouldDelete := false
		if err := item.Value(func(val []byte) error {
			var tv taskValue
			if err := json.Unmarshal(val, &tv); err != nil {
				return err
			}
			shouldDelete = tv.ShareGroup == shareGroup && tv.SharedTaskID != ""
			return nil
		}); err != nil {
			return err
		}
		if shouldDelete {
			key := item.KeyCopy(nil)
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
	}
	return nil
}
