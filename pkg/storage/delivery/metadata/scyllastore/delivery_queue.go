package scyllastore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

const defaultReadBucketLimit = 128

func (s *DeliveryQueueStore) AppendDeliveryTask(ctx context.Context, task brokerstore.DeliveryTask) (bool, error) {
	if s == nil || s.session == nil {
		return false, fmt.Errorf("scylla session is nil")
	}
	if task.ClientID == "" {
		return false, fmt.Errorf("clientID is empty")
	}
	if task.TaskID == uuid.Nil {
		return false, fmt.Errorf("taskID is empty")
	}
	if task.MessageID == uuid.Nil {
		return false, fmt.Errorf("messageID is empty")
	}
	if task.TS.IsZero() {
		task.TS = time.Now()
	}

	generation, err := s.readClientGeneration(ctx, task.ClientID)
	if err != nil {
		return false, fmt.Errorf("read client generation: %w", err)
	}
	bucketStart := bucketStartNano(task.TS, s.bucketDuration)
	taskID, err := toCQLUUID(task.TaskID)
	if err != nil {
		return false, err
	}
	messageID, err := toCQLUUID(task.MessageID)
	if err != nil {
		return false, err
	}

	applied, err := s.insertClientTaskDedupe(ctx, task, generation, bucketStart, taskID, messageID)
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}
	if err := s.insertClientTaskRows(ctx, task, generation, bucketStart, taskID, messageID); err != nil {
		_ = s.deleteClientTaskDedupe(ctx, task.ClientID, messageID)
		return false, err
	}
	return true, nil
}

func (s *DeliveryQueueStore) DeliveryTaskExists(ctx context.Context, clientID string, messageID uuid.UUID) (bool, error) {
	if s == nil || s.session == nil {
		return false, fmt.Errorf("scylla session is nil")
	}
	if clientID == "" || messageID == uuid.Nil {
		return false, nil
	}
	cqlMessageID, err := toCQLUUID(messageID)
	if err != nil {
		return false, err
	}
	generation, err := s.readClientGeneration(ctx, clientID)
	if err != nil {
		return false, err
	}
	var taskID gocql.UUID
	err = s.session.Query(
		`SELECT task_id FROM delivery_task_dedupe_by_client WHERE client_id = ? AND generation = ? AND message_id = ? LIMIT 1`,
		clientID,
		generation,
		cqlMessageID,
	).WithContext(ctx).Scan(&taskID)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *DeliveryQueueStore) ReadDeliveryTasks(
	ctx context.Context,
	clientID string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
	limit int,
) ([]*brokerstore.DeliveryTask, error) {
	if s == nil || s.session == nil {
		return nil, fmt.Errorf("scylla session is nil")
	}
	if clientID == "" || limit <= 0 {
		return nil, nil
	}
	generation, err := s.readClientGeneration(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("read client generation: %w", err)
	}
	startBucket := int64(0)
	if !lastTS.IsZero() {
		startBucket = bucketStartNano(lastTS, s.bucketDuration)
	}
	buckets, err := s.readClientBuckets(ctx, clientID, generation, startBucket)
	if err != nil {
		return nil, err
	}

	out := make([]*brokerstore.DeliveryTask, 0, limit)
	for _, bucket := range buckets {
		bucketLastTS := int64(0)
		bucketLastTaskID := uuid.Nil
		if bucket == startBucket {
			bucketLastTS = lastTS.UnixNano()
			bucketLastTaskID = lastTaskID
		}
		tasks, err := s.readClientTasksFromBucket(ctx, clientID, generation, bucket, bucketLastTS, bucketLastTaskID, limit-len(out))
		if err != nil {
			return nil, err
		}
		out = append(out, tasks...)
		if len(out) >= limit {
			return out, nil
		}
	}
	return out, nil
}

func (s *DeliveryQueueStore) AdvanceDeliveryCursor(ctx context.Context, cursor brokerstore.DeliveryCursor) (bool, error) {
	if s == nil || s.session == nil {
		return false, fmt.Errorf("scylla session is nil")
	}
	if cursor.ClientID == "" {
		return false, fmt.Errorf("clientID is empty")
	}
	if cursor.LastTaskID == uuid.Nil {
		return false, fmt.Errorf("lastTaskID is empty")
	}
	taskID, err := toCQLUUID(cursor.LastTaskID)
	if err != nil {
		return false, err
	}
	current, err := s.ReadDeliveryCursor(ctx, cursor.ClientID)
	if err != nil {
		return false, err
	}
	if !deliveryCursorAfter(cursor, current) {
		return false, nil
	}
	updated := cursor.UpdatedTS
	if updated.IsZero() {
		updated = time.Now()
	}
	if current == nil {
		applied, err := s.session.Query(
			`INSERT INTO delivery_client_state (client_id, generation, last_ts_nano, last_task_id, updated_at) VALUES (?, ?, ?, ?, ?) IF NOT EXISTS`,
			cursor.ClientID,
			cursor.Generation,
			cursor.LastTS.UnixNano(),
			taskID,
			updated.UTC(),
		).WithContext(ctx).MapScanCAS(map[string]any{})
		if err != nil {
			return false, err
		}
		return applied, nil
	}
	currentTaskID, err := toCQLUUID(current.LastTaskID)
	if err != nil {
		return false, err
	}
	applied, err := s.session.Query(
		`UPDATE delivery_client_state SET generation = ?, last_ts_nano = ?, last_task_id = ?, updated_at = ? WHERE client_id = ? IF generation = ? AND last_ts_nano = ? AND last_task_id = ?`,
		cursor.Generation,
		cursor.LastTS.UnixNano(),
		taskID,
		updated.UTC(),
		cursor.ClientID,
		current.Generation,
		current.LastTS.UnixNano(),
		currentTaskID,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *DeliveryQueueStore) ReadDeliveryCursor(ctx context.Context, clientID string) (*brokerstore.DeliveryCursor, error) {
	if s == nil || s.session == nil {
		return nil, fmt.Errorf("scylla session is nil")
	}
	if clientID == "" {
		return nil, nil
	}
	var (
		updatedAt  time.Time
		generation int64
		lastTSNano int64
		taskID     gocql.UUID
	)
	err := s.session.Query(
		`SELECT updated_at, generation, last_ts_nano, last_task_id FROM delivery_client_state WHERE client_id = ? LIMIT 1`,
		clientID,
	).WithContext(ctx).Scan(&updatedAt, &generation, &lastTSNano, &taskID)
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &brokerstore.DeliveryCursor{
		UpdatedTS:  updatedAt,
		ClientID:   clientID,
		Generation: generation,
		LastTS:     time.Unix(0, lastTSNano),
		LastTaskID: fromCQLUUID(taskID),
	}, nil
}

func (s *DeliveryQueueStore) ResetClientDeliveryState(ctx context.Context, clientID string) error {
	if s == nil || s.session == nil {
		return fmt.Errorf("scylla session is nil")
	}
	if clientID == "" {
		return fmt.Errorf("clientID is empty")
	}
	generation, err := s.readClientGeneration(ctx, clientID)
	if err != nil {
		return err
	}
	err = s.session.Query(
		`UPDATE delivery_client_state SET generation = ?, last_ts_nano = ?, last_task_id = ?, updated_at = ? WHERE client_id = ?`,
		generation+1,
		int64(0),
		gocql.UUID{},
		time.Now().UTC(),
		clientID,
	).WithContext(ctx).Exec()
	if err != nil {
		return err
	}
	return nil
}

func deliveryCursorAfter(next brokerstore.DeliveryCursor, current *brokerstore.DeliveryCursor) bool {
	if current == nil {
		return true
	}
	if next.Generation != current.Generation {
		return false
	}
	nextTS := next.LastTS.UnixNano()
	currentTS := current.LastTS.UnixNano()
	if nextTS != currentTS {
		return nextTS > currentTS
	}
	return bytes.Compare(next.LastTaskID[:], current.LastTaskID[:]) > 0
}

func (s *DeliveryQueueStore) readClientGeneration(ctx context.Context, clientID string) (int64, error) {
	var generation int64
	err := s.session.Query(
		`SELECT generation FROM delivery_client_state WHERE client_id = ? LIMIT 1`,
		clientID,
	).WithContext(ctx).Scan(&generation)
	if errors.Is(err, errNotFound) {
		return 0, nil
	}
	return generation, err
}

func (s *DeliveryQueueStore) insertClientTaskDedupe(
	ctx context.Context,
	task brokerstore.DeliveryTask,
	generation int64,
	bucketStart int64,
	taskID gocql.UUID,
	messageID gocql.UUID,
) (bool, error) {
	applied, err := s.session.Query(
		`INSERT INTO delivery_task_dedupe_by_client (client_id, generation, message_id, bucket_start_nano, ts_nano, task_id) VALUES (?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
		task.ClientID,
		generation,
		messageID,
		bucketStart,
		task.TS.UnixNano(),
		taskID,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return false, fmt.Errorf("insert delivery task dedupe: %w", err)
	}
	return applied, nil
}

func (s *DeliveryQueueStore) insertClientTaskRows(
	ctx context.Context,
	task brokerstore.DeliveryTask,
	generation int64,
	bucketStart int64,
	taskID gocql.UUID,
	messageID gocql.UUID,
) error {
	if err := s.session.Query(
		`INSERT INTO delivery_client_buckets (client_id, generation, bucket_start_nano) VALUES (?, ?, ?)`,
		task.ClientID,
		generation,
		bucketStart,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("insert delivery client bucket: %w", err)
	}
	sharedTaskID := (*gocql.UUID)(nil)
	if task.SharedTaskID != uuid.Nil {
		cqlSharedTaskID, err := toCQLUUID(task.SharedTaskID)
		if err != nil {
			return err
		}
		sharedTaskID = &cqlSharedTaskID
	}
	if err := s.session.Query(
		`INSERT INTO delivery_tasks_by_client_bucket (client_id, generation, bucket_start_nano, ts_nano, task_id, message_id, delivery_qos, subscription_ids, no_local, retain_as_published, share_group, shared_task_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ClientID,
		generation,
		bucketStart,
		task.TS.UnixNano(),
		taskID,
		messageID,
		task.DeliveryQoS,
		task.SubscriptionIDs,
		task.NoLocal,
		task.RetainAsPublished,
		task.ShareGroup,
		sharedTaskID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("insert delivery task: %w", err)
	}
	return nil
}

func (s *DeliveryQueueStore) deleteClientTaskDedupe(ctx context.Context, clientID string, messageID gocql.UUID) error {
	generation, err := s.readClientGeneration(ctx, clientID)
	if err != nil {
		return err
	}
	return s.session.Query(
		`DELETE FROM delivery_task_dedupe_by_client WHERE client_id = ? AND generation = ? AND message_id = ?`,
		clientID,
		generation,
		messageID,
	).WithContext(ctx).Exec()
}

func (s *DeliveryQueueStore) readClientBuckets(
	ctx context.Context,
	clientID string,
	generation int64,
	startBucket int64,
) ([]int64, error) {
	iter := s.session.Query(
		`SELECT bucket_start_nano FROM delivery_client_buckets WHERE client_id = ? AND generation = ? AND bucket_start_nano >= ? LIMIT ?`,
		clientID,
		generation,
		startBucket,
		defaultReadBucketLimit,
	).WithContext(ctx).Iter()
	buckets := make([]int64, 0, 8)
	var bucket int64
	for iter.Scan(&bucket) {
		buckets = append(buckets, bucket)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return buckets, nil
}

func (s *DeliveryQueueStore) readClientTasksFromBucket(
	ctx context.Context,
	clientID string,
	generation int64,
	bucketStart int64,
	lastTSNano int64,
	lastTaskID uuid.UUID,
	limit int,
) ([]*brokerstore.DeliveryTask, error) {
	if limit <= 0 {
		return nil, nil
	}
	cqlLastTaskID, err := toCQLUUID(lastTaskID)
	if err != nil {
		return nil, err
	}
	iter := s.session.Query(
		`SELECT ts_nano, task_id, message_id, delivery_qos, subscription_ids, no_local, retain_as_published, share_group, shared_task_id FROM delivery_tasks_by_client_bucket WHERE client_id = ? AND generation = ? AND bucket_start_nano = ? AND (ts_nano, task_id) > (?, ?) LIMIT ?`,
		clientID,
		generation,
		bucketStart,
		lastTSNano,
		cqlLastTaskID,
		limit,
	).WithContext(ctx).Iter()
	tasks := make([]*brokerstore.DeliveryTask, 0, limit)
	for {
		task, ok := scanDeliveryTask(iter, clientID, generation)
		if !ok {
			break
		}
		tasks = append(tasks, task)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func scanDeliveryTask(iter cqlIter, clientID string, generation int64) (*brokerstore.DeliveryTask, bool) {
	var (
		tsNano             int64
		taskID             gocql.UUID
		messageID          gocql.UUID
		deliveryQoS        int
		subscriptionIDs    []int32
		noLocal            bool
		retainAsPublished  bool
		shareGroup         string
		sharedTaskID       gocql.UUID
		sharedTaskIDIsNull bool
	)
	if !iter.Scan(
		&tsNano,
		&taskID,
		&messageID,
		&deliveryQoS,
		&subscriptionIDs,
		&noLocal,
		&retainAsPublished,
		&shareGroup,
		&sharedTaskID,
	) {
		return nil, false
	}
	if sharedTaskID == (gocql.UUID{}) {
		sharedTaskIDIsNull = true
	}
	task := &brokerstore.DeliveryTask{
		TS:                time.Unix(0, tsNano),
		TaskID:            fromCQLUUID(taskID),
		ClientID:          clientID,
		MessageID:         fromCQLUUID(messageID),
		Generation:        generation,
		DeliveryQoS:       deliveryQoS,
		SubscriptionIDs:   subscriptionIDs,
		NoLocal:           noLocal,
		RetainAsPublished: retainAsPublished,
		ShareGroup:        shareGroup,
	}
	if !sharedTaskIDIsNull {
		task.SharedTaskID = fromCQLUUID(sharedTaskID)
	}
	return task, true
}

func toCQLUUID(id uuid.UUID) (gocql.UUID, error) {
	if id == uuid.Nil {
		return gocql.UUID{}, nil
	}
	out, err := gocql.ParseUUID(id.String())
	if err != nil {
		return gocql.UUID{}, fmt.Errorf("parse uuid: %w", err)
	}
	return out, nil
}

func fromCQLUUID(id gocql.UUID) uuid.UUID {
	if id == (gocql.UUID{}) {
		return uuid.Nil
	}
	out, err := uuid.Parse(id.String())
	if err != nil {
		return uuid.Nil
	}
	return out
}
