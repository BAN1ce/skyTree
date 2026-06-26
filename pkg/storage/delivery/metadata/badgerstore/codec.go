package badgerstore

import (
	"encoding/json"
	"fmt"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
)

type opType string

const (
	opEnsureSchema           opType = "ensure_schema"
	opAppendTask             opType = "append_task"
	opAdvanceCursor          opType = "append_cursor"
	opDeleteClientState      opType = "delete_client_state"
	opDeleteSharedTasks      opType = "delete_shared_tasks"
	opAppendShareTask        opType = "append_share_task"
	opUpdateShareTaskStatus  opType = "update_share_task_status"
	opAppendShareGroupCursor opType = "append_share_group_cursor"
	opRollbackShareGroupTask opType = "rollback_share_group_task"
)

type updateRequest struct {
	Op opType `json:"op"`
	// Data is one of:
	// - appendTaskRequest
	// - advanceCursorRequest
	Data json.RawMessage `json:"data,omitempty"`
}

type appendTaskRequest struct {
	TSUnixNano        int64   `json:"ts_unix_nano"`
	TaskID            string  `json:"task_id"`
	ClientID          string  `json:"client_id"`
	MessageID         string  `json:"message_id"`
	DeliveryQoS       int     `json:"delivery_qos"`
	SubscriptionIDs   []int32 `json:"subscription_ids,omitempty"`
	NoLocal           bool    `json:"no_local"`
	RetainAsPublished bool    `json:"retain_as_published"`
	ShareGroup        string  `json:"share_group,omitempty"`
	SharedTaskID      string  `json:"shared_task_id,omitempty"`
}

type advanceCursorRequest struct {
	UpdatedTSUnixNano int64  `json:"updated_ts_unix_nano"`
	ClientID          string `json:"client_id"`
	LastTSUnixNano    int64  `json:"last_ts_unix_nano"`
	LastTaskID        string `json:"last_task_id"`
}

type deleteClientStateRequest struct {
	ClientID string `json:"client_id"`
}

type deleteSharedTasksRequest struct {
	ClientID   string `json:"client_id"`
	ShareGroup string `json:"share_group"`
}

type appendShareTaskRequest struct {
	TSUnixNano      int64                         `json:"ts_unix_nano"`
	TaskID          string                        `json:"task_id"`
	ShareGroup      string                        `json:"share_group"`
	TopicFilter     string                        `json:"topic_filter"`
	MessageID       string                        `json:"message_id"`
	DeliveryQoS     int                           `json:"delivery_qos"`
	PublishQoS      int                           `json:"publish_qos"`
	PublisherClient string                        `json:"publisher_client,omitempty"`
	SubscriptionIDs string                        `json:"subscription_ids,omitempty"`
	WinnerNoLocal   bool                          `json:"winner_no_local"`
	WinnerRAP       bool                          `json:"winner_rap"`
	Status          sharedsubscription.TaskStatus `json:"status"`
	ProcessedByNode uint64                        `json:"processed_by_node"`
	ProcessedAtNano int64                         `json:"processed_at_nano"`
	RollbackReason  string                        `json:"rollback_reason,omitempty"`
}

type updateShareTaskStatusRequest struct {
	TaskID     string                        `json:"task_id"`
	ShareGroup string                        `json:"share_group"`
	OldStatus  sharedsubscription.TaskStatus `json:"old_status"`
	NewStatus  sharedsubscription.TaskStatus `json:"new_status"`
	NowNano    int64                         `json:"now_nano"`
}

type appendShareGroupCursorRequest struct {
	ShareGroup          string `json:"share_group"`
	LastProcessedTSNano int64  `json:"last_processed_ts_nano"`
	LastProcessedTaskID string `json:"last_processed_task_id"`
	LeaderNodeID        uint64 `json:"leader_node_id"`
	LastRenewalNano     int64  `json:"last_renewal_nano"`
}

func marshalUpdate(op opType, v any) ([]byte, error) {
	var raw json.RawMessage
	if v != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(updateRequest{Op: op, Data: raw})
}

func unmarshalUpdate(b []byte) (*updateRequest, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty update bytes")
	}
	var req updateRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, err
	}
	if req.Op == "" {
		return nil, fmt.Errorf("missing op")
	}
	return &req, nil
}
