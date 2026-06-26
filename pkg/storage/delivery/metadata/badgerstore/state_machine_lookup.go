package badgerstore

import (
	"bytes"
	"encoding/json"
	"time"

	broker_store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
)

func (s *DeliveryStateMachine) lookupDeliveryBacklogSummary() (*broker_store.DeliveryBacklogSummary, error) {
	summary := &broker_store.DeliveryBacklogSummary{}
	activeClients := make(map[string]struct{})

	err := s.db.View(func(txn *badger.Txn) error {
		itOpt := badger.DefaultIteratorOptions
		itOpt.PrefetchValues = false
		it := txn.NewIterator(itOpt)
		defer it.Close()

		for it.Seek(prefixTask); it.ValidForPrefix(prefixTask); it.Next() {
			summary.PendingTasks++
			if clientID, ok := parseClientIDFromTaskKey(it.Item().Key()); ok {
				activeClients[clientID] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	summary.ActiveClients = int64(len(activeClients))
	return summary, nil
}

func parseClientIDFromTaskKey(key []byte) (string, bool) {
	if !bytes.HasPrefix(key, prefixTask) {
		return "", false
	}
	rest := key[len(prefixTask):]
	delimiterIndex := bytes.IndexByte(rest, 0)
	if delimiterIndex <= 0 {
		return "", false
	}
	return string(rest[:delimiterIndex]), true
}

func (s *DeliveryStateMachine) lookupDeliveryCursor(q *readCursorQuery) (*broker_store.DeliveryCursor, error) {
	if q == nil || q.ClientID == "" {
		return nil, nil
	}
	var out *broker_store.DeliveryCursor
	err := s.db.View(func(txn *badger.Txn) error {
		cursor, err := lookupDeliveryCursorInTxn(txn, q.ClientID)
		if err != nil {
			return err
		}
		out = cursor
		return nil
	})
	return out, err
}

func lookupDeliveryCursorInTxn(txn *badger.Txn, clientID string) (*broker_store.DeliveryCursor, error) {
	item, err := txn.Get(cursorKey(clientID))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out *broker_store.DeliveryCursor
	err = item.Value(func(val []byte) error {
		cursor, err := deliveryCursorFromValue(val)
		if err != nil {
			return err
		}
		out = cursor
		return nil
	})
	return out, err
}

func deliveryPositionAfter(tsUnixNano int64, taskID uuid.UUID, current *broker_store.DeliveryCursor) bool {
	if current == nil {
		return true
	}
	if tsUnixNano != current.LastTS.UnixNano() {
		return tsUnixNano > current.LastTS.UnixNano()
	}
	return bytes.Compare(taskID[:], current.LastTaskID[:]) > 0
}

func deliveryCursorFromValue(val []byte) (*broker_store.DeliveryCursor, error) {
	var cv cursorValue
	if err := json.Unmarshal(val, &cv); err != nil {
		return nil, err
	}
	lastTask, err := uuid.Parse(cv.LastTaskID)
	if err != nil {
		return nil, err
	}
	return &broker_store.DeliveryCursor{
		UpdatedTS:  time.Unix(0, cv.UpdatedTSUnixNano),
		ClientID:   cv.ClientID,
		LastTS:     time.Unix(0, cv.LastTSUnixNano),
		LastTaskID: lastTask,
	}, nil
}

func (s *DeliveryStateMachine) lookupDeliveryTasks(q *readTasksQuery) ([]*broker_store.DeliveryTask, error) {
	if q == nil || q.ClientID == "" || q.Limit <= 0 {
		return nil, nil
	}
	clientPrefix := taskKeyPrefix(q.ClientID)
	seekKey := deliveryTaskSeekKey(q, clientPrefix)
	out := make([]*broker_store.DeliveryTask, 0, q.Limit)
	err := s.db.View(func(txn *badger.Txn) error {
		itOpt := badger.DefaultIteratorOptions
		itOpt.PrefetchValues = true
		it := txn.NewIterator(itOpt)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(clientPrefix) && len(out) < q.Limit; it.Next() {
			task, ok, err := deliveryTaskFromItem(q.ClientID, clientPrefix, seekKey, len(out), it.Item())
			if err != nil {
				return err
			}
			if ok {
				out = append(out, task)
			}
		}
		return nil
	})
	return out, err
}

func deliveryTaskSeekKey(q *readTasksQuery, clientPrefix []byte) []byte {
	if q.LastTaskID == uuid.Nil && q.LastTSUnixNano <= 0 {
		return clientPrefix
	}
	lastTaskBytes := [16]byte(q.LastTaskID)
	return taskKey(q.ClientID, q.LastTSUnixNano, lastTaskBytes)
}

func deliveryTaskFromItem(
	clientID string,
	clientPrefix []byte,
	seekKey []byte,
	outLen int,
	item *badger.Item,
) (*broker_store.DeliveryTask, bool, error) {
	key := item.Key()
	if outLen == 0 && bytes.Equal(key, seekKey) {
		return nil, false, nil
	}
	tsUnixNano, taskIDBytes, ok := parseTaskKey(key, clientPrefix)
	if !ok {
		return nil, false, nil
	}
	taskUUID, err := uuid.FromBytes(taskIDBytes[:])
	if err != nil {
		return nil, false, nil
	}
	var tv taskValue
	if err := item.Value(func(val []byte) error {
		return json.Unmarshal(val, &tv)
	}); err != nil {
		return nil, false, err
	}
	task, ok := deliveryTaskFromValue(clientID, tsUnixNano, taskUUID, tv)
	return task, ok, nil
}

func deliveryTaskFromValue(clientID string, tsUnixNano int64, taskUUID uuid.UUID, tv taskValue) (*broker_store.DeliveryTask, bool) {
	msgUUID, err := uuid.Parse(tv.MessageID)
	if err != nil {
		return nil, false
	}
	sharedTaskID := uuid.Nil
	if tv.SharedTaskID != "" {
		parsed, err := uuid.Parse(tv.SharedTaskID)
		if err == nil {
			sharedTaskID = parsed
		}
	}
	return &broker_store.DeliveryTask{
		TS:                time.Unix(0, tsUnixNano),
		TaskID:            taskUUID,
		ClientID:          clientID,
		MessageID:         msgUUID,
		DeliveryQoS:       tv.DeliveryQoS,
		SubscriptionIDs:   cloneSubscriptionIDs(tv.SubscriptionIDs),
		NoLocal:           tv.NoLocal,
		RetainAsPublished: tv.RetainAsPublished,
		ShareGroup:        tv.ShareGroup,
		SharedTaskID:      sharedTaskID,
	}, true
}

func cloneSubscriptionIDs(ids []int32) []int32 {
	if len(ids) > 0 {
		out := make([]int32, len(ids))
		copy(out, ids)
		return out
	}
	return nil
}
