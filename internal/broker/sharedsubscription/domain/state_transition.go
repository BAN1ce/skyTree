package domain

import "github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"

func CanTransitionTaskStatus(from, to sharedsubscription.TaskStatus) bool {
	switch from {
	case sharedsubscription.TaskStatusPending:
		return to == sharedsubscription.TaskStatusProcessing
	case sharedsubscription.TaskStatusProcessing:
		return to == sharedsubscription.TaskStatusPending ||
			to == sharedsubscription.TaskStatusCompleted ||
			to == sharedsubscription.TaskStatusRolledBack
	default:
		return false
	}
}
