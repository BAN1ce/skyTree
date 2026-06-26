package domain

import (
	"testing"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
)

func TestCanTransitionTaskStatus(t *testing.T) {
	tests := []struct {
		name string
		from sharedsubscription.TaskStatus
		to   sharedsubscription.TaskStatus
		want bool
	}{
		{name: "pending_to_processing", from: sharedsubscription.TaskStatusPending, to: sharedsubscription.TaskStatusProcessing, want: true},
		{name: "processing_to_pending", from: sharedsubscription.TaskStatusProcessing, to: sharedsubscription.TaskStatusPending, want: true},
		{name: "processing_to_completed", from: sharedsubscription.TaskStatusProcessing, to: sharedsubscription.TaskStatusCompleted, want: true},
		{name: "processing_to_rolled_back", from: sharedsubscription.TaskStatusProcessing, to: sharedsubscription.TaskStatusRolledBack, want: true},
		{name: "pending_to_completed_invalid", from: sharedsubscription.TaskStatusPending, to: sharedsubscription.TaskStatusCompleted, want: false},
		{name: "completed_to_pending_invalid", from: sharedsubscription.TaskStatusCompleted, to: sharedsubscription.TaskStatusPending, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanTransitionTaskStatus(tc.from, tc.to); got != tc.want {
				t.Fatalf("CanTransitionTaskStatus(%s,%s)=%v want=%v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}
