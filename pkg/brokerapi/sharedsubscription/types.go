package sharedsubscription

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the status of a shared subscription task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusRolledBack TaskStatus = "rolled_back"
)

// ShareGroupTask represents a task in the shared subscription queue
type ShareGroupTask struct {
	TaskID          uuid.UUID
	ShareGroup      string
	TopicFilter     string
	MessageID       uuid.UUID
	DeliveryQoS     int
	PublishQoS      int
	PublisherClient string
	SubscriptionIDs string // JSON array string
	WinnerNoLocal   bool
	WinnerRAP       bool
	Status          TaskStatus
	ProcessedByNode uint64
	ProcessedAt     time.Time
	RollbackReason  string
	Timestamp       time.Time
}

// ShareGroupMember represents a member in a shared subscription group
type ShareGroupMember struct {
	ShareGroup  string
	ClientID    string
	TopicFilter string
}

// ShareGroupCursor represents the consumption cursor for a shared subscription group
type ShareGroupCursor struct {
	ShareGroup          string
	LastProcessedTS     time.Time
	LastProcessedTaskID uuid.UUID
	LeaderNodeID        uint64
	LastRenewal         time.Time
}
