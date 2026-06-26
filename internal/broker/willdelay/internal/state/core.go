package state

import (
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/indexedheap"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"google.golang.org/protobuf/proto"
)

// Core owns the in-memory will-delay task state and expiration index.
type Core struct {
	mux sync.RWMutex

	// Scheduler index for querying due tasks without scanning all task state.
	dueTasks *indexedheap.Queue[string, *proto_will_delay.WillDelayTask]

	// Persistent task state. The key is ClientID or ClientID+OwnerToken so stale owners cannot
	// overwrite newer will-delay tasks for the same client.
	tasks map[string]*proto_will_delay.WillDelayTask
}

const willDelayTaskKeySeparator = "\x00"

// NewCore creates a Core with an empty runtime scheduler.
func NewCore(interval time.Duration, slotNum int) *Core {
	_, _ = interval, slotNum
	c := &Core{
		dueTasks: indexedheap.New[string, *proto_will_delay.WillDelayTask](),
		tasks:    make(map[string]*proto_will_delay.WillDelayTask),
	}
	return c
}

// AddTask adds or replaces a will-delay task for the same client owner.
func (c *Core) AddTask(task *proto_will_delay.WillDelayTask) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if err := ValidateTaskForAdd(task); err != nil {
		logger.Logger.Error().Err(err).Msg("invalid will delay task")
		return err
	}
	key := taskKey(task)
	clonedTask := proto.Clone(task).(*proto_will_delay.WillDelayTask)

	c.tasks[key] = clonedTask
	c.dueTasks.Upsert(key, clonedTask.GetScheduledPublishTime(), clonedTask)
	return nil
}

// DeleteTask deletes all will-delay tasks for a client.
func (c *Core) DeleteTask(clientID string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for key, task := range c.tasks {
		if task == nil || task.GetClientID() != clientID {
			continue
		}
		c.deleteTaskKeyUnsafe(key)
	}
}

// DeleteTaskByOwner deletes one owner-token variant without touching newer owners.
func (c *Core) DeleteTaskByOwner(clientID, ownerToken string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if ownerToken == "" {
		for key, task := range c.tasks {
			if task == nil || task.GetClientID() != clientID {
				continue
			}
			c.deleteTaskKeyUnsafe(key)
		}
		return
	}
	c.deleteTaskKeyUnsafe(taskKeyFromParts(clientID, ownerToken))
}

// deleteTaskKeyUnsafe deletes a task by state key. The caller must hold c.mux.
func (c *Core) deleteTaskKeyUnsafe(key string) {
	delete(c.tasks, key)
	c.dueTasks.Remove(key)
}

// GetDueTasks returns tasks due at or before nowUnixMicro without deleting them.
func (c *Core) GetDueTasks(nowUnixMicro int64) []*proto_will_delay.WillDelayTask {
	c.mux.RLock()
	defer c.mux.RUnlock()

	var dueTasks []*proto_will_delay.WillDelayTask
	for _, task := range c.dueTasks.ValuesAtOrBefore(nowUnixMicro) {
		if task != nil {
			dueTasks = append(dueTasks, proto.Clone(task).(*proto_will_delay.WillDelayTask))
		}
	}

	return dueTasks
}

// GetState returns a clone of persistent state for snapshotting.
func (c *Core) GetState() *proto_will_delay.WillDelayState {
	c.mux.RLock()
	defer c.mux.RUnlock()

	state := &proto_will_delay.WillDelayState{
		Tasks: make(map[string]*proto_will_delay.WillDelayTask),
	}

	for k, v := range c.tasks {
		state.Tasks[k] = proto.Clone(v).(*proto_will_delay.WillDelayTask)
	}

	return state
}

// RecoverFromState restores persistent task state and rebuilds the runtime expiration index.
func (c *Core) RecoverFromState(state *proto_will_delay.WillDelayState) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.tasks = make(map[string]*proto_will_delay.WillDelayTask)
	c.dueTasks = indexedheap.New[string, *proto_will_delay.WillDelayTask]()

	for _, task := range state.GetTasks() {
		if err := ValidateTaskForAdd(task); err != nil {
			logger.Logger.Warn().Err(err).Msg("skip invalid will delay task during recovery")
			continue
		}
		key := taskKey(task)
		clonedTask := proto.Clone(task).(*proto_will_delay.WillDelayTask)
		c.tasks[key] = clonedTask
		c.dueTasks.Upsert(key, clonedTask.GetScheduledPublishTime(), clonedTask)
	}

	logger.Logger.Info().
		Int("task_count", len(c.tasks)).
		Msg("Recovered will delay tasks from snapshot")
}

func taskKey(task *proto_will_delay.WillDelayTask) string {
	if task == nil {
		return ""
	}
	return taskKeyFromParts(task.GetClientID(), task.GetOwnerToken())
}

func taskKeyFromParts(clientID, ownerToken string) string {
	if ownerToken == "" {
		return clientID
	}
	return clientID + willDelayTaskKeySeparator + ownerToken
}
