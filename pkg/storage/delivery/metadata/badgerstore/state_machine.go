package badgerstore

import (
	"fmt"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/cluster/healthcheck"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
	"github.com/lni/dragonboat/v3/statemachine"
)

type taskValue struct {
	MessageID         string  `json:"message_id"`
	DeliveryQoS       int     `json:"delivery_qos"`
	SubscriptionIDs   []int32 `json:"subscription_ids,omitempty"`
	NoLocal           bool    `json:"no_local"`
	RetainAsPublished bool    `json:"retain_as_published"`
	ShareGroup        string  `json:"share_group,omitempty"`
	SharedTaskID      string  `json:"shared_task_id,omitempty"`
}

type cursorValue struct {
	UpdatedTSUnixNano int64  `json:"updated_ts_unix_nano"`
	ClientID          string `json:"client_id"`
	LastTSUnixNano    int64  `json:"last_ts_unix_nano"`
	LastTaskID        string `json:"last_task_id"`
}

type shareTaskValue struct {
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

type shareCursorValue struct {
	ShareGroup          string `json:"share_group"`
	LastProcessedTSNano int64  `json:"last_processed_ts_nano"`
	LastProcessedTaskID string `json:"last_processed_task_id"`
	LeaderNodeID        uint64 `json:"leader_node_id"`
	LastRenewalNano     int64  `json:"last_renewal_nano"`
}

type readTasksQuery struct {
	ClientID       string
	LastTSUnixNano int64
	LastTaskID     uuid.UUID
	Limit          int
}

type readCursorQuery struct {
	ClientID string
}

type readBacklogSummaryQuery struct{}

type readShareTasksQuery struct {
	ShareGroup     string
	LastTSUnixNano int64
	LastTaskID     uuid.UUID
	Limit          int
}

type queryProcessingShareTasksBeforeQuery struct {
	ShareGroup string
	BeforeNano int64
}

type readShareCursorQuery struct {
	ShareGroup string
}

type queryShareTaskByMessageIDQuery struct {
	ShareGroup string
	MessageID  uuid.UUID
	Statuses   []sharedsubscription.TaskStatus
}

// DeliveryStateMachine is a Dragonboat state machine that stores delivery queue metadata in Badger.
// It is designed to be used per Raft group (shard).
type DeliveryStateMachine struct {
	db *badger.DB
}

func NewDeliveryStateMachine(db *badger.DB) *DeliveryStateMachine {
	return &DeliveryStateMachine{db: db}
}

func (s *DeliveryStateMachine) Update(b []byte) (statemachine.Result, error) {
	if ignoreDeliveryStateUpdate(b) {
		return statemachine.Result{}, nil
	}

	req, err := unmarshalUpdate(b)
	if err != nil {
		return statemachine.Result{}, err
	}
	return s.applyUpdate(req)
}

func ignoreDeliveryStateUpdate(b []byte) bool {
	return len(b) == 0 || string(b) == healthcheck.HealthCheckMessage
}

func (s *DeliveryStateMachine) applyUpdate(req *updateRequest) (statemachine.Result, error) {
	switch req.Op {
	case opEnsureSchema:
		// KV store: nothing to do.
		return statemachine.Result{}, nil
	case opAppendTask:
		return s.updateAppendTask(req)
	case opAdvanceCursor:
		return s.updateAdvanceCursor(req)
	case opDeleteClientState:
		return s.updateDeleteClientState(req)
	case opDeleteSharedTasks:
		return s.updateDeleteSharedTasks(req)
	case opAppendShareTask:
		return s.updateAppendShareTask(req)
	case opUpdateShareTaskStatus:
		return s.updateShareTaskStatus(req)
	case opRollbackShareGroupTask:
		return s.updateRollbackShareGroupTask(req)
	case opAppendShareGroupCursor:
		return s.updateAppendShareGroupCursor(req)
	default:
		return statemachine.Result{}, fmt.Errorf("unknown op %q", req.Op)
	}
}

func (s *DeliveryStateMachine) Lookup(q interface{}) (interface{}, error) {
	switch v := q.(type) {
	case *readCursorQuery:
		return s.lookupDeliveryCursor(v)
	case *readBacklogSummaryQuery:
		return s.lookupDeliveryBacklogSummary()
	case *readTasksQuery:
		return s.lookupDeliveryTasks(v)
	case *readShareTasksQuery:
		if v == nil || v.ShareGroup == "" || v.Limit <= 0 {
			return ([]*sharedsubscription.ShareGroupTask)(nil), nil
		}
		return s.readShareGroupTasks(v)
	case *queryProcessingShareTasksBeforeQuery:
		if v == nil || v.ShareGroup == "" {
			return ([]*sharedsubscription.ShareGroupTask)(nil), nil
		}
		return s.queryProcessingShareTasksBefore(v)
	case *readShareCursorQuery:
		if v == nil || v.ShareGroup == "" {
			return (*sharedsubscription.ShareGroupCursor)(nil), nil
		}
		return s.readShareGroupCursor(v.ShareGroup)
	case *queryShareTaskByMessageIDQuery:
		if v == nil || v.ShareGroup == "" || v.MessageID == uuid.Nil {
			return (*sharedsubscription.ShareGroupTask)(nil), nil
		}
		return s.queryShareTaskByMessageID(v)
	default:
		logger.Logger.Error().Msg("delivery state machine: invalid lookup type")
		return nil, fmt.Errorf("invalid lookup type %T", q)
	}
}
