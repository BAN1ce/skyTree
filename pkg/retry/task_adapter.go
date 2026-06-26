package retry

import (
	"time"

	"github.com/BAN1ce/skyTree/pkg/scheduler"
)

// TaskAdapter adapts retry.Task into scheduler.Task for retry scheduling.
type TaskAdapter struct {
	task       *Task
	expireTime int64 // 过期时间（微秒时间戳）
}

// NewTaskAdapter 创建新的任务适配器
func NewTaskAdapter(task *Task, delay time.Duration) *TaskAdapter {
	expireTime := time.Now().Add(delay).UnixMicro()

	return &TaskAdapter{
		task:       task,
		expireTime: expireTime,
	}
}

// GetKey 获取任务唯一标识
func (a *TaskAdapter) GetKey() string {
	if a.task == nil {
		return ""
	}
	return a.task.Key
}

// GetExpireTime 获取过期时间（微秒时间戳）
func (a *TaskAdapter) GetExpireTime() int64 {
	return a.expireTime
}

// GetTask 获取原始 Task
func (a *TaskAdapter) GetTask() *Task {
	return a.task
}

// Ensure TaskAdapter implements scheduler.Task.
var _ scheduler.Task = (*TaskAdapter)(nil)
