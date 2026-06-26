package domain

import (
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
)

type AssignTaskCommand struct {
	ShareGroup      string
	TopicFilter     string
	PublisherClient string
	PublishQoS      int
	Task            *sharedsubscription.ShareGroupTask
}

type ResolveClientOptionsCommand struct {
	ShareGroup      string
	TopicFilter     string
	PublisherClient string
	PublishQoS      int
	ClientID        string
}

type RollbackCommand struct {
	ClientID    string
	ShareGroup  string
	Delay       time.Duration
	Reason      string
	AllowOnline bool
}
