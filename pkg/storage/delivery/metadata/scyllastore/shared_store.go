package scyllastore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

const sharedDeliveryReadBatchSize = 200

type shareTaskRow struct {
	TaskID          uuid.UUID
	ShareGroup      string
	TopicFilter     string
	MessageID       uuid.UUID
	DeliveryQoS     int
	SubscriptionIDs string
	WinnerNoLocal   bool
	WinnerRAP       bool
	Status          sharedsubscription.TaskStatus
	ProcessedByNode uint64
	ProcessedAtNano int64
	RollbackReason  string
	TSNano          int64
	BucketStartNano int64
}

func (s *DeliveryQueueStore) AppendShareGroupTask(
	ctx context.Context,
	ts time.Time,
	task *sharedsubscription.ShareGroupTask,
) error {
	row, err := s.shareTaskRowFromTask(ts, task)
	if err != nil {
		return err
	}
	applied, err := s.session.Query(
		`INSERT INTO share_group_task_by_id (share_group, task_id, status, bucket_start_nano, ts_nano, topic_filter, message_id, delivery_qos, subscription_ids, winner_no_local, winner_rap, processed_by_node, processed_at_nano, rollback_reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
		row.ShareGroup,
		mustCQLUUID(row.TaskID),
		statusString(row.Status),
		row.BucketStartNano,
		row.TSNano,
		row.TopicFilter,
		mustCQLUUID(row.MessageID),
		row.DeliveryQoS,
		row.SubscriptionIDs,
		row.WinnerNoLocal,
		row.WinnerRAP,
		int64(row.ProcessedByNode),
		row.ProcessedAtNano,
		row.RollbackReason,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return fmt.Errorf("insert share task: %w", err)
	}
	if !applied {
		return nil
	}
	return s.insertShareTaskIndexes(ctx, row)
}

func (s *DeliveryQueueStore) ReadShareGroupTasks(
	ctx context.Context,
	shareGroup string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
	limit int,
) ([]*sharedsubscription.ShareGroupTask, error) {
	if shareGroup == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.readShareRowsByStatus(ctx, shareGroup, sharedsubscription.TaskStatusPending, lastTS, lastTaskID, limit, time.Time{})
	if err != nil {
		return nil, err
	}
	out := make([]*sharedsubscription.ShareGroupTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, shareTaskFromRow(row))
	}
	return out, nil
}

func (s *DeliveryQueueStore) MarkShareGroupTaskProcessed(
	ctx context.Context,
	taskID uuid.UUID,
	shareGroup string,
) error {
	for _, oldStatus := range []sharedsubscription.TaskStatus{
		sharedsubscription.TaskStatusProcessing,
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusRolledBack,
	} {
		updated, err := s.AtomicUpdateTaskStatus(
			ctx,
			taskID,
			shareGroup,
			oldStatus,
			sharedsubscription.TaskStatusCompleted,
		)
		if err != nil || updated {
			return err
		}
	}
	return nil
}

func (s *DeliveryQueueStore) AtomicUpdateTaskStatus(
	ctx context.Context,
	taskID uuid.UUID,
	shareGroup string,
	oldStatus sharedsubscription.TaskStatus,
	newStatus sharedsubscription.TaskStatus,
) (bool, error) {
	row, err := s.readShareTaskByID(ctx, shareGroup, taskID)
	if err != nil || row == nil {
		return false, err
	}
	if row.Status != oldStatus {
		return false, nil
	}
	next := *row
	nowNano := time.Now().UnixNano()
	next.Status = newStatus
	switch newStatus {
	case sharedsubscription.TaskStatusPending:
		next.TSNano = nowNano
		next.BucketStartNano = bucketStartNano(time.Unix(0, nowNano), s.bucketDuration)
		next.ProcessedAtNano = 0
		next.ProcessedByNode = 0
	case sharedsubscription.TaskStatusProcessing, sharedsubscription.TaskStatusCompleted, sharedsubscription.TaskStatusRolledBack:
		next.ProcessedAtNano = nowNano
	}
	applied, err := s.updateShareTaskStatusCAS(ctx, *row, next)
	if err != nil || !applied {
		return applied, err
	}
	if err := s.deleteShareTaskIndexes(ctx, *row); err != nil {
		return false, err
	}
	if err := s.insertShareTaskIndexes(ctx, next); err != nil {
		return false, err
	}
	return true, nil
}

func (s *DeliveryQueueStore) GetUnAckedSharedSubscriptionTasks(
	ctx context.Context,
	clientID string,
	shareGroup string,
	lastTS time.Time,
	lastTaskID uuid.UUID,
) ([]*sharedsubscription.ShareGroupTask, error) {
	out := make([]*sharedsubscription.ShareGroupTask, 0)
	for {
		tasks, err := s.ReadDeliveryTasks(ctx, clientID, lastTS, lastTaskID, sharedDeliveryReadBatchSize)
		if err != nil {
			return nil, err
		}
		if len(tasks) == 0 {
			return out, nil
		}
		for _, task := range tasks {
			if task == nil {
				continue
			}
			lastTS = task.TS
			lastTaskID = task.TaskID
			if task.ShareGroup != shareGroup || task.SharedTaskID == uuid.Nil {
				continue
			}
			out = append(out, shareGroupTaskFromDeliveryTask(task))
		}
		if len(tasks) < sharedDeliveryReadBatchSize {
			return out, nil
		}
	}
}

func (s *DeliveryQueueStore) RollbackSharedSubscriptionTask(ctx context.Context, task *sharedsubscription.ShareGroupTask) error {
	if task == nil {
		return fmt.Errorf("shared subscription task is nil")
	}
	existing, err := s.readShareTaskByID(ctx, task.ShareGroup, task.TaskID)
	if err != nil {
		return err
	}
	if existing == nil {
		task.Status = sharedsubscription.TaskStatusPending
		return s.AppendShareGroupTask(ctx, time.Now(), task)
	}
	if existing.Status == sharedsubscription.TaskStatusCompleted {
		return nil
	}
	_, err = s.AtomicUpdateTaskStatus(ctx, task.TaskID, task.ShareGroup, existing.Status, sharedsubscription.TaskStatusPending)
	return err
}

func (s *DeliveryQueueStore) QueryProcessingTasksBefore(
	ctx context.Context,
	shareGroup string,
	beforeTime time.Time,
) ([]*sharedsubscription.ShareGroupTask, error) {
	if shareGroup == "" {
		return nil, nil
	}
	rows, err := s.readShareRowsByStatus(ctx, shareGroup, sharedsubscription.TaskStatusProcessing, time.Time{}, uuid.Nil, 0, beforeTime)
	if err != nil {
		return nil, err
	}
	out := make([]*sharedsubscription.ShareGroupTask, 0, len(rows))
	for _, row := range rows {
		if beforeTime.IsZero() || row.ProcessedAtNano <= beforeTime.UnixNano() {
			out = append(out, shareTaskFromRow(row))
		}
	}
	return out, nil
}

func (s *DeliveryQueueStore) ReadShareGroupCursor(
	ctx context.Context,
	shareGroup string,
) (*sharedsubscription.ShareGroupCursor, error) {
	if shareGroup == "" {
		return nil, nil
	}
	var (
		lastProcessedNano int64
		taskID            gocql.UUID
		leaderNodeID      int64
		lastRenewal       time.Time
	)
	err := s.session.Query(
		`SELECT last_processed_ts_nano, last_processed_task_id, leader_node_id, last_renewal FROM share_group_cursor WHERE share_group = ? LIMIT 1`,
		shareGroup,
	).WithContext(ctx).Scan(&lastProcessedNano, &taskID, &leaderNodeID, &lastRenewal)
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sharedsubscription.ShareGroupCursor{
		ShareGroup:          shareGroup,
		LastProcessedTS:     time.Unix(0, lastProcessedNano),
		LastProcessedTaskID: fromCQLUUID(taskID),
		LeaderNodeID:        uint64(leaderNodeID),
		LastRenewal:         lastRenewal,
	}, nil
}

func (s *DeliveryQueueStore) AppendShareGroupCursor(
	ctx context.Context,
	shareGroup string,
	cursor *sharedsubscription.ShareGroupCursor,
) error {
	if cursor == nil {
		return fmt.Errorf("share group cursor is nil")
	}
	if shareGroup == "" {
		shareGroup = cursor.ShareGroup
	}
	if shareGroup == "" {
		return fmt.Errorf("shareGroup is empty")
	}
	taskID, err := toCQLUUID(cursor.LastProcessedTaskID)
	if err != nil {
		return err
	}
	return s.session.Query(
		`UPDATE share_group_cursor SET last_processed_ts_nano = ?, last_processed_task_id = ?, leader_node_id = ?, last_renewal = ? WHERE share_group = ?`,
		cursor.LastProcessedTS.UnixNano(),
		taskID,
		int64(cursor.LeaderNodeID),
		cursor.LastRenewal.UTC(),
		shareGroup,
	).WithContext(ctx).Exec()
}

func (s *DeliveryQueueStore) QueryShareGroupTaskByMessageID(
	ctx context.Context,
	shareGroup string,
	messageID uuid.UUID,
	statuses []sharedsubscription.TaskStatus,
) (*sharedsubscription.ShareGroupTask, error) {
	if shareGroup == "" || messageID == uuid.Nil {
		return nil, nil
	}
	messageCQL, err := toCQLUUID(messageID)
	if err != nil {
		return nil, err
	}
	statusValues := statuses
	if len(statusValues) == 0 {
		statusValues = []sharedsubscription.TaskStatus{
			sharedsubscription.TaskStatusPending,
			sharedsubscription.TaskStatusProcessing,
			sharedsubscription.TaskStatusRolledBack,
			sharedsubscription.TaskStatusCompleted,
		}
	}
	for _, status := range statusValues {
		var taskID gocql.UUID
		err := s.session.Query(
			`SELECT task_id FROM share_group_task_by_message WHERE share_group = ? AND message_id = ? AND status = ? LIMIT 1`,
			shareGroup,
			messageCQL,
			statusString(status),
		).WithContext(ctx).Scan(&taskID)
		if errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		row, err := s.readShareTaskByID(ctx, shareGroup, fromCQLUUID(taskID))
		if err != nil || row == nil {
			return nil, err
		}
		return shareTaskFromRow(*row), nil
	}
	return nil, nil
}

func (s *DeliveryQueueStore) shareTaskRowFromTask(ts time.Time, task *sharedsubscription.ShareGroupTask) (shareTaskRow, error) {
	if task == nil {
		return shareTaskRow{}, fmt.Errorf("shared subscription task is nil")
	}
	if task.TaskID == uuid.Nil {
		return shareTaskRow{}, fmt.Errorf("taskID is empty")
	}
	if task.ShareGroup == "" {
		return shareTaskRow{}, fmt.Errorf("shareGroup is empty")
	}
	if task.MessageID == uuid.Nil {
		return shareTaskRow{}, fmt.Errorf("messageID is empty")
	}
	if ts.IsZero() {
		ts = task.Timestamp
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	var processedAtNano int64
	if !task.ProcessedAt.IsZero() {
		processedAtNano = task.ProcessedAt.UnixNano()
	}
	return shareTaskRow{
		TaskID:          task.TaskID,
		ShareGroup:      task.ShareGroup,
		TopicFilter:     task.TopicFilter,
		MessageID:       task.MessageID,
		DeliveryQoS:     task.DeliveryQoS,
		SubscriptionIDs: task.SubscriptionIDs,
		WinnerNoLocal:   task.WinnerNoLocal,
		WinnerRAP:       task.WinnerRAP,
		Status:          sharedsubscription.TaskStatus(statusString(task.Status)),
		ProcessedByNode: task.ProcessedByNode,
		ProcessedAtNano: processedAtNano,
		RollbackReason:  task.RollbackReason,
		TSNano:          ts.UnixNano(),
		BucketStartNano: bucketStartNano(ts, s.bucketDuration),
	}, nil
}

func (s *DeliveryQueueStore) insertShareTaskIndexes(ctx context.Context, row shareTaskRow) error {
	status := statusString(row.Status)
	taskID := mustCQLUUID(row.TaskID)
	messageID := mustCQLUUID(row.MessageID)
	if err := s.session.Query(
		`INSERT INTO share_group_buckets (share_group, status, bucket_start_nano) VALUES (?, ?, ?)`,
		row.ShareGroup,
		status,
		row.BucketStartNano,
	).WithContext(ctx).Exec(); err != nil {
		return err
	}
	if err := s.session.Query(
		`INSERT INTO share_group_tasks_by_status_bucket (share_group, status, bucket_start_nano, ts_nano, task_id, topic_filter, message_id, delivery_qos, subscription_ids, winner_no_local, winner_rap, processed_by_node, processed_at_nano, rollback_reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ShareGroup,
		status,
		row.BucketStartNano,
		row.TSNano,
		taskID,
		row.TopicFilter,
		messageID,
		row.DeliveryQoS,
		row.SubscriptionIDs,
		row.WinnerNoLocal,
		row.WinnerRAP,
		int64(row.ProcessedByNode),
		row.ProcessedAtNano,
		row.RollbackReason,
	).WithContext(ctx).Exec(); err != nil {
		return err
	}
	return s.session.Query(
		`INSERT INTO share_group_task_by_message (share_group, message_id, status, task_id) VALUES (?, ?, ?, ?)`,
		row.ShareGroup,
		messageID,
		status,
		taskID,
	).WithContext(ctx).Exec()
}

func (s *DeliveryQueueStore) deleteShareTaskIndexes(ctx context.Context, row shareTaskRow) error {
	status := statusString(row.Status)
	taskID := mustCQLUUID(row.TaskID)
	messageID := mustCQLUUID(row.MessageID)
	if err := s.session.Query(
		`DELETE FROM share_group_tasks_by_status_bucket WHERE share_group = ? AND status = ? AND bucket_start_nano = ? AND ts_nano = ? AND task_id = ?`,
		row.ShareGroup,
		status,
		row.BucketStartNano,
		row.TSNano,
		taskID,
	).WithContext(ctx).Exec(); err != nil {
		return err
	}
	return s.session.Query(
		`DELETE FROM share_group_task_by_message WHERE share_group = ? AND message_id = ? AND status = ? AND task_id = ?`,
		row.ShareGroup,
		messageID,
		status,
		taskID,
	).WithContext(ctx).Exec()
}

func (s *DeliveryQueueStore) updateShareTaskStatusCAS(ctx context.Context, oldRow shareTaskRow, next shareTaskRow) (bool, error) {
	applied, err := s.session.Query(
		`UPDATE share_group_task_by_id SET status = ?, bucket_start_nano = ?, ts_nano = ?, processed_by_node = ?, processed_at_nano = ?, rollback_reason = ? WHERE share_group = ? AND task_id = ? IF status = ?`,
		statusString(next.Status),
		next.BucketStartNano,
		next.TSNano,
		int64(next.ProcessedByNode),
		next.ProcessedAtNano,
		next.RollbackReason,
		next.ShareGroup,
		mustCQLUUID(next.TaskID),
		statusString(oldRow.Status),
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *DeliveryQueueStore) readShareTaskByID(ctx context.Context, shareGroup string, taskID uuid.UUID) (*shareTaskRow, error) {
	if shareGroup == "" || taskID == uuid.Nil {
		return nil, nil
	}
	row := shareTaskRow{ShareGroup: shareGroup, TaskID: taskID}
	var (
		status          string
		messageID       gocql.UUID
		processedByNode int64
	)
	err := s.session.Query(
		`SELECT status, bucket_start_nano, ts_nano, topic_filter, message_id, delivery_qos, subscription_ids, winner_no_local, winner_rap, processed_by_node, processed_at_nano, rollback_reason FROM share_group_task_by_id WHERE share_group = ? AND task_id = ? LIMIT 1`,
		shareGroup,
		mustCQLUUID(taskID),
	).WithContext(ctx).Scan(
		&status,
		&row.BucketStartNano,
		&row.TSNano,
		&row.TopicFilter,
		&messageID,
		&row.DeliveryQoS,
		&row.SubscriptionIDs,
		&row.WinnerNoLocal,
		&row.WinnerRAP,
		&processedByNode,
		&row.ProcessedAtNano,
		&row.RollbackReason,
	)
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.Status = sharedsubscription.TaskStatus(status)
	row.MessageID = fromCQLUUID(messageID)
	row.ProcessedByNode = uint64(processedByNode)
	return &row, nil
}

func (s *DeliveryQueueStore) readShareRowsByStatus(
	ctx context.Context,
	shareGroup string,
	status sharedsubscription.TaskStatus,
	lastTS time.Time,
	lastTaskID uuid.UUID,
	limit int,
	before time.Time,
) ([]shareTaskRow, error) {
	startBucket := int64(0)
	if !lastTS.IsZero() {
		startBucket = bucketStartNano(lastTS, s.bucketDuration)
	}
	buckets, err := s.readShareBuckets(ctx, shareGroup, status, startBucket)
	if err != nil {
		return nil, err
	}
	effectiveLimit := limit
	if effectiveLimit <= 0 {
		effectiveLimit = sharedDeliveryReadBatchSize
	}
	out := make([]shareTaskRow, 0, effectiveLimit)
	for _, bucket := range buckets {
		bucketLastTS := int64(0)
		bucketLastTaskID := uuid.Nil
		if bucket == startBucket {
			bucketLastTS = lastTS.UnixNano()
			bucketLastTaskID = lastTaskID
		}
		rows, err := s.readShareRowsFromBucket(ctx, shareGroup, status, bucket, bucketLastTS, bucketLastTaskID, effectiveLimit-len(out))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if !before.IsZero() && row.ProcessedAtNano > before.UnixNano() {
				continue
			}
			out = append(out, row)
		}
		if len(out) >= effectiveLimit {
			return out, nil
		}
	}
	return out, nil
}

func (s *DeliveryQueueStore) readShareBuckets(
	ctx context.Context,
	shareGroup string,
	status sharedsubscription.TaskStatus,
	startBucket int64,
) ([]int64, error) {
	iter := s.session.Query(
		`SELECT bucket_start_nano FROM share_group_buckets WHERE share_group = ? AND status = ? AND bucket_start_nano >= ? LIMIT ?`,
		shareGroup,
		statusString(status),
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

func (s *DeliveryQueueStore) readShareRowsFromBucket(
	ctx context.Context,
	shareGroup string,
	status sharedsubscription.TaskStatus,
	bucketStart int64,
	lastTSNano int64,
	lastTaskID uuid.UUID,
	limit int,
) ([]shareTaskRow, error) {
	if limit <= 0 {
		return nil, nil
	}
	iter := s.session.Query(
		`SELECT ts_nano, task_id, topic_filter, message_id, delivery_qos, subscription_ids, winner_no_local, winner_rap, processed_by_node, processed_at_nano, rollback_reason FROM share_group_tasks_by_status_bucket WHERE share_group = ? AND status = ? AND bucket_start_nano = ? AND (ts_nano, task_id) > (?, ?) LIMIT ?`,
		shareGroup,
		statusString(status),
		bucketStart,
		lastTSNano,
		mustCQLUUID(lastTaskID),
		limit,
	).WithContext(ctx).Iter()
	rows := make([]shareTaskRow, 0, limit)
	for {
		row, ok := scanShareTaskRow(iter, shareGroup, status, bucketStart)
		if !ok {
			break
		}
		rows = append(rows, row)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return rows, nil
}

func scanShareTaskRow(iter cqlIter, shareGroup string, status sharedsubscription.TaskStatus, bucketStart int64) (shareTaskRow, bool) {
	var (
		row             = shareTaskRow{ShareGroup: shareGroup, Status: status, BucketStartNano: bucketStart}
		taskID          gocql.UUID
		messageID       gocql.UUID
		processedByNode int64
	)
	if !iter.Scan(
		&row.TSNano,
		&taskID,
		&row.TopicFilter,
		&messageID,
		&row.DeliveryQoS,
		&row.SubscriptionIDs,
		&row.WinnerNoLocal,
		&row.WinnerRAP,
		&processedByNode,
		&row.ProcessedAtNano,
		&row.RollbackReason,
	) {
		return shareTaskRow{}, false
	}
	row.TaskID = fromCQLUUID(taskID)
	row.MessageID = fromCQLUUID(messageID)
	row.ProcessedByNode = uint64(processedByNode)
	return row, true
}

func shareTaskFromRow(row shareTaskRow) *sharedsubscription.ShareGroupTask {
	return &sharedsubscription.ShareGroupTask{
		TaskID:          row.TaskID,
		ShareGroup:      row.ShareGroup,
		TopicFilter:     row.TopicFilter,
		MessageID:       row.MessageID,
		DeliveryQoS:     row.DeliveryQoS,
		SubscriptionIDs: row.SubscriptionIDs,
		WinnerNoLocal:   row.WinnerNoLocal,
		WinnerRAP:       row.WinnerRAP,
		Status:          row.Status,
		ProcessedByNode: row.ProcessedByNode,
		ProcessedAt:     time.Unix(0, row.ProcessedAtNano),
		RollbackReason:  row.RollbackReason,
		Timestamp:       time.Unix(0, row.TSNano),
	}
}

func shareGroupTaskFromDeliveryTask(task *brokerstore.DeliveryTask) *sharedsubscription.ShareGroupTask {
	taskID := task.SharedTaskID
	if taskID == uuid.Nil {
		taskID = task.TaskID
	}
	return &sharedsubscription.ShareGroupTask{
		TaskID:          taskID,
		ShareGroup:      task.ShareGroup,
		MessageID:       task.MessageID,
		DeliveryQoS:     task.DeliveryQoS,
		SubscriptionIDs: subscriptionIDsJSON(task.SubscriptionIDs),
		WinnerNoLocal:   task.NoLocal,
		WinnerRAP:       task.RetainAsPublished,
		Timestamp:       task.TS,
	}
}

func subscriptionIDsJSON(ids []int32) string {
	if len(ids) == 0 {
		return ""
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(b)
}

func mustCQLUUID(id uuid.UUID) gocql.UUID {
	out, err := toCQLUUID(id)
	if err != nil {
		return gocql.UUID{}
	}
	return out
}
