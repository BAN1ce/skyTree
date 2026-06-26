package state

import (
	"fmt"

	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

// ValidateTaskForAdd validates a task before it can enter any will-delay store.
func ValidateTaskForAdd(task *proto_will_delay.WillDelayTask) error {
	if task == nil {
		return fmt.Errorf("nil will delay task")
	}
	if task.GetClientID() == "" {
		return fmt.Errorf("will delay task client id is required")
	}
	if task.GetOwnerToken() == "" {
		return fmt.Errorf("will delay task owner token is required")
	}
	return nil
}
