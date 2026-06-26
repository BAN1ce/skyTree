package badgerstore

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
)

func (s *DeliveryStateMachine) readShareGroupTasks(q *readShareTasksQuery) ([]*sharedsubscription.ShareGroupTask, error) {
	groupPrefix := shareTaskKeyPrefix(q.ShareGroup)
	lastTaskBytes := [16]byte(q.LastTaskID)
	seekKey := shareTaskKey(q.ShareGroup, q.LastTSUnixNano, lastTaskBytes)
	if q.LastTaskID == uuid.Nil && q.LastTSUnixNano <= 0 {
		seekKey = groupPrefix
	}

	out := make([]*sharedsubscription.ShareGroupTask, 0, q.Limit)
	err := s.db.View(func(txn *badger.Txn) error {
		itOpt := badger.DefaultIteratorOptions
		itOpt.PrefetchValues = true
		it := txn.NewIterator(itOpt)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(groupPrefix) && len(out) < q.Limit; it.Next() {
			item := it.Item()
			key := item.Key()
			if len(out) == 0 && bytes.Equal(key, seekKey) {
				continue
			}
			tsUnixNano, taskIDBytes, ok := parseShareTaskKey(key, groupPrefix)
			if !ok {
				continue
			}
			taskUUID, err := uuid.FromBytes(taskIDBytes[:])
			if err != nil {
				continue
			}
			var tv shareTaskValue
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &tv)
			}); err != nil {
				return err
			}
			if tv.Status != sharedsubscription.TaskStatusPending {
				continue
			}
			task, ok := shareTaskFromValue(q.ShareGroup, tsUnixNano, taskUUID, tv)
			if ok {
				out = append(out, task)
			}
		}
		return nil
	})
	return out, err
}

func (s *DeliveryStateMachine) queryProcessingShareTasksBefore(q *queryProcessingShareTasksBeforeQuery) ([]*sharedsubscription.ShareGroupTask, error) {
	groupPrefix := shareTaskKeyPrefix(q.ShareGroup)
	out := make([]*sharedsubscription.ShareGroupTask, 0)
	err := s.db.View(func(txn *badger.Txn) error {
		itOpt := badger.DefaultIteratorOptions
		itOpt.PrefetchValues = true
		it := txn.NewIterator(itOpt)
		defer it.Close()

		for it.Seek(groupPrefix); it.ValidForPrefix(groupPrefix); it.Next() {
			item := it.Item()
			tsUnixNano, taskIDBytes, ok := parseShareTaskKey(item.Key(), groupPrefix)
			if !ok {
				continue
			}
			taskUUID, err := uuid.FromBytes(taskIDBytes[:])
			if err != nil {
				continue
			}
			var tv shareTaskValue
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &tv)
			}); err != nil {
				return err
			}
			if tv.Status != sharedsubscription.TaskStatusProcessing {
				continue
			}
			if tv.ProcessedAtNano > q.BeforeNano {
				continue
			}
			task, ok := shareTaskFromValue(q.ShareGroup, tsUnixNano, taskUUID, tv)
			if ok {
				out = append(out, task)
			}
		}
		return nil
	})
	return out, err
}

func (s *DeliveryStateMachine) readShareGroupCursor(shareGroup string) (*sharedsubscription.ShareGroupCursor, error) {
	key := shareCursorKey(shareGroup)
	var out *sharedsubscription.ShareGroupCursor
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			var cv shareCursorValue
			if err := json.Unmarshal(val, &cv); err != nil {
				return err
			}
			lastTaskID := uuid.Nil
			if cv.LastProcessedTaskID != "" {
				parsed, err := uuid.Parse(cv.LastProcessedTaskID)
				if err != nil {
					return err
				}
				lastTaskID = parsed
			}
			out = &sharedsubscription.ShareGroupCursor{
				ShareGroup:          cv.ShareGroup,
				LastProcessedTS:     time.Unix(0, cv.LastProcessedTSNano),
				LastProcessedTaskID: lastTaskID,
				LeaderNodeID:        cv.LeaderNodeID,
				LastRenewal:         time.Unix(0, cv.LastRenewalNano),
			}
			return nil
		})
	})
	return out, err
}

func (s *DeliveryStateMachine) queryShareTaskByMessageID(q *queryShareTaskByMessageIDQuery) (*sharedsubscription.ShareGroupTask, error) {
	groupPrefix := shareTaskKeyPrefix(q.ShareGroup)
	statuses := make(map[sharedsubscription.TaskStatus]struct{}, len(q.Statuses))
	for _, status := range q.Statuses {
		statuses[status] = struct{}{}
	}
	var out *sharedsubscription.ShareGroupTask
	err := s.db.View(func(txn *badger.Txn) error {
		itOpt := badger.DefaultIteratorOptions
		itOpt.PrefetchValues = true
		it := txn.NewIterator(itOpt)
		defer it.Close()

		for it.Seek(groupPrefix); it.ValidForPrefix(groupPrefix); it.Next() {
			item := it.Item()
			tsUnixNano, taskIDBytes, ok := parseShareTaskKey(item.Key(), groupPrefix)
			if !ok {
				continue
			}
			taskUUID, err := uuid.FromBytes(taskIDBytes[:])
			if err != nil {
				continue
			}
			var tv shareTaskValue
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &tv)
			}); err != nil {
				return err
			}
			if len(statuses) > 0 {
				if _, ok := statuses[tv.Status]; !ok {
					continue
				}
			}
			if tv.MessageID != q.MessageID.String() {
				continue
			}
			task, ok := shareTaskFromValue(q.ShareGroup, tsUnixNano, taskUUID, tv)
			if ok {
				out = task
				return nil
			}
		}
		return nil
	})
	return out, err
}

func shareTaskFromValue(shareGroup string, tsUnixNano int64, taskID uuid.UUID, tv shareTaskValue) (*sharedsubscription.ShareGroupTask, bool) {
	messageID, err := uuid.Parse(tv.MessageID)
	if err != nil {
		return nil, false
	}
	return &sharedsubscription.ShareGroupTask{
		TaskID:          taskID,
		ShareGroup:      shareGroup,
		TopicFilter:     tv.TopicFilter,
		MessageID:       messageID,
		DeliveryQoS:     tv.DeliveryQoS,
		PublishQoS:      tv.PublishQoS,
		PublisherClient: tv.PublisherClient,
		SubscriptionIDs: tv.SubscriptionIDs,
		WinnerNoLocal:   tv.WinnerNoLocal,
		WinnerRAP:       tv.WinnerRAP,
		Status:          tv.Status,
		ProcessedByNode: tv.ProcessedByNode,
		ProcessedAt:     time.Unix(0, tv.ProcessedAtNano),
		RollbackReason:  tv.RollbackReason,
		Timestamp:       time.Unix(0, tsUnixNano),
	}, true
}

func updateShareTaskStatus(
	txn *badger.Txn,
	shareGroup string,
	taskID uuid.UUID,
	oldStatus sharedsubscription.TaskStatus,
	newStatus sharedsubscription.TaskStatus,
	nowNano int64,
) (bool, error) {
	if nowNano <= 0 {
		nowNano = time.Now().UnixNano()
	}
	key, tsUnixNano, tv, ok, err := findShareTask(txn, shareGroup, taskID)
	if err != nil || !ok {
		return false, err
	}
	if tv.Status != oldStatus {
		return false, nil
	}
	return setShareTaskStatus(txn, key, shareGroup, taskID, tsUnixNano, tv, newStatus, nowNano)
}

func rollbackShareTask(txn *badger.Txn, req appendShareTaskRequest) error {
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return err
	}
	nowNano := req.TSUnixNano
	if nowNano <= 0 {
		nowNano = time.Now().UnixNano()
	}
	key, tsUnixNano, tv, ok, err := findShareTask(txn, req.ShareGroup, taskID)
	if err != nil {
		return err
	}
	if ok {
		if tv.Status == sharedsubscription.TaskStatusCompleted {
			return nil
		}
		_, err = setShareTaskStatus(txn, key, req.ShareGroup, taskID, tsUnixNano, tv, sharedsubscription.TaskStatusPending, nowNano)
		return err
	}
	taskBytes := [16]byte(taskID)
	val, err := json.Marshal(shareTaskValue{
		ShareGroup:      req.ShareGroup,
		TopicFilter:     req.TopicFilter,
		MessageID:       req.MessageID,
		DeliveryQoS:     req.DeliveryQoS,
		PublishQoS:      req.PublishQoS,
		PublisherClient: req.PublisherClient,
		SubscriptionIDs: req.SubscriptionIDs,
		WinnerNoLocal:   req.WinnerNoLocal,
		WinnerRAP:       req.WinnerRAP,
		Status:          sharedsubscription.TaskStatusPending,
		RollbackReason:  req.RollbackReason,
	})
	if err != nil {
		return err
	}
	return txn.Set(shareTaskKey(req.ShareGroup, nowNano, taskBytes), val)
}

func setShareTaskStatus(
	txn *badger.Txn,
	oldKey []byte,
	shareGroup string,
	taskID uuid.UUID,
	tsUnixNano int64,
	tv shareTaskValue,
	newStatus sharedsubscription.TaskStatus,
	nowNano int64,
) (bool, error) {
	newTS := tsUnixNano
	tv.Status = newStatus
	switch newStatus {
	case sharedsubscription.TaskStatusPending:
		newTS = nowNano
		tv.ProcessedAtNano = 0
		tv.ProcessedByNode = 0
	case sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusCompleted, sharedsubscription.TaskStatusRolledBack:
		tv.ProcessedAtNano = nowNano
	}
	val, err := json.Marshal(tv)
	if err != nil {
		return false, err
	}
	taskBytes := [16]byte(taskID)
	newKey := shareTaskKey(shareGroup, newTS, taskBytes)
	if !bytes.Equal(oldKey, newKey) {
		if err := txn.Delete(oldKey); err != nil {
			return false, err
		}
	}
	return true, txn.Set(newKey, val)
}

func findShareTask(
	txn *badger.Txn,
	shareGroup string,
	taskID uuid.UUID,
) (key []byte, tsUnixNano int64, value shareTaskValue, ok bool, err error) {
	groupPrefix := shareTaskKeyPrefix(shareGroup)
	want := [16]byte(taskID)
	itOpt := badger.DefaultIteratorOptions
	itOpt.PrefetchValues = true
	it := txn.NewIterator(itOpt)
	defer it.Close()

	for it.Seek(groupPrefix); it.ValidForPrefix(groupPrefix); it.Next() {
		item := it.Item()
		ts, taskIDBytes, parsed := parseShareTaskKey(item.Key(), groupPrefix)
		if !parsed || taskIDBytes != want {
			continue
		}
		var tv shareTaskValue
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tv)
		}); err != nil {
			return nil, 0, shareTaskValue{}, false, err
		}
		return item.KeyCopy(nil), ts, tv, true, nil
	}
	return nil, 0, shareTaskValue{}, false, nil
}
