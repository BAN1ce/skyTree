package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/domain"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	shared "github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription/selector"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/google/uuid"
)

const (
	defaultBatchSize         = 100
	defaultProcessingTimeout = 30 * time.Second
	defaultWakeInterval      = 5 * time.Second
)

// SharedSubscriptionConsumer consumes messages from shared subscription queue and distributes to clients
type SharedSubscriptionConsumer struct {
	shareGroup    string
	topicFilter   string
	store         store.SharedSubscriptionStore
	taskStore     delivery.TaskStore
	clientManager ClientManagerInterface
	selector      selector.ShareGroupSelector
	strategy      domain.SharedDeliveryStrategy
	subCenter     domain.SubCenter
	cursorStore   store.SharedSubscriptionStore
	rollbackSvc   *domain.RollbackService
	localNodeID   uint64
	deliveryEvent delivery_notify.ClientDeliveryEvent

	ctx     context.Context
	cancel  context.CancelFunc
	runMu   sync.Mutex
	running bool
	wg      sync.WaitGroup
	wakeCh  chan struct{}

	wakeClientFunc func(string)

	lastProcessedTS     time.Time
	lastProcessedTaskID uuid.UUID
	mu                  sync.RWMutex
}

// NewSharedSubscriptionConsumer creates a new shared subscription consumer
// ClientManagerInterface defines the interface for client manager
type ClientManagerInterface interface {
	ReadClient(id string) (interface{}, bool)
}

func NewSharedSubscriptionConsumer(
	shareGroup string,
	topicFilter string,
	store store.SharedSubscriptionStore,
	taskStore delivery.TaskStore,
	clientManager ClientManagerInterface,
	selector selector.ShareGroupSelector,
	subCenter domain.SubCenter,
	strategy domain.SharedDeliveryStrategy,
	rollbackSvc *domain.RollbackService,
	localNodeID uint64,
	deliveryEvent delivery_notify.ClientDeliveryEvent,
) *SharedSubscriptionConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	if strategy == nil {
		strategy = domain.NewSharedDeliveryStrategy(subCenter, nil, clientManager)
	}
	if rollbackSvc == nil {
		rollbackSvc = domain.NewRollbackService(store, nil, nil)
	}
	return &SharedSubscriptionConsumer{
		shareGroup:    shareGroup,
		topicFilter:   topicFilter,
		store:         store,
		taskStore:     taskStore,
		clientManager: clientManager,
		selector:      selector,
		strategy:      strategy,
		subCenter:     subCenter,
		cursorStore:   store,
		rollbackSvc:   rollbackSvc,
		localNodeID:   localNodeID,
		deliveryEvent: deliveryEvent,
		ctx:           ctx,
		cancel:        cancel,
		wakeCh:        make(chan struct{}, 1),
	}
}

// Run starts the consumer loop and binds it to the provided lifecycle context.
func (c *SharedSubscriptionConsumer) Run(ctx context.Context) {
	if c == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)

	c.runMu.Lock()
	if c.running {
		c.runMu.Unlock()
		cancel()
		return
	}
	c.ctx = runCtx
	c.cancel = cancel
	c.running = true
	c.wg.Add(2)
	c.runMu.Unlock()

	go c.run()
	go c.scanTimeoutTasks()
}

// Stop stops the consumer
func (c *SharedSubscriptionConsumer) Stop() {
	if c == nil {
		return
	}
	c.runMu.Lock()
	cancel := c.cancel
	c.runMu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
}

func (c *SharedSubscriptionConsumer) run() {
	defer c.wg.Done()
	defer func() {
		c.runMu.Lock()
		c.running = false
		c.runMu.Unlock()
	}()

	// Load cursor
	c.loadCursor()

	// Main consumption loop
	ticker := time.NewTicker(defaultWakeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.wakeCh:
			c.processTasks()
		case <-ticker.C:
			c.processTasks()
		}
	}
}

func (c *SharedSubscriptionConsumer) Wake() {
	if c == nil || c.wakeCh == nil {
		return
	}
	select {
	case c.wakeCh <- struct{}{}:
	default:
	}
}

func (c *SharedSubscriptionConsumer) loadCursor() {
	cursor, err := c.cursorStore.ReadShareGroupCursor(c.ctx, c.shareGroup)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("shareGroup", c.shareGroup).Msg("failed to read cursor")
		return
	}
	if cursor != nil {
		c.mu.Lock()
		c.lastProcessedTS = cursor.LastProcessedTS
		c.lastProcessedTaskID = cursor.LastProcessedTaskID
		c.mu.Unlock()
	}
}

func (c *SharedSubscriptionConsumer) processTasks() {
	c.mu.RLock()
	lastTS := c.lastProcessedTS
	lastTaskID := c.lastProcessedTaskID
	c.mu.RUnlock()

	tasks, err := c.store.ReadShareGroupTasks(c.ctx, c.shareGroup, lastTS, lastTaskID, defaultBatchSize)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("shareGroup", c.shareGroup).Msg("failed to read tasks")
		return
	}

	if len(tasks) == 0 {
		return
	}

	for i, task := range tasks {
		if c.ctx.Err() != nil {
			return
		}

		if err := c.processTaskAt(task, i); err != nil {
			logger.Logger.Warn().Err(err).Str("shareGroup", c.shareGroup).Str("taskID", task.TaskID.String()).Msg("failed to process task")
			// Continue processing other tasks
			continue
		}

		// Update cursor
		c.mu.Lock()
		c.lastProcessedTS = task.Timestamp
		c.lastProcessedTaskID = task.TaskID
		c.mu.Unlock()

		// Update cursor in store
		cursor := &shared.ShareGroupCursor{
			ShareGroup:          c.shareGroup,
			LastProcessedTS:     task.Timestamp,
			LastProcessedTaskID: task.TaskID,
		}
		_ = c.cursorStore.AppendShareGroupCursor(c.ctx, c.shareGroup, cursor)
	}
}

func (c *SharedSubscriptionConsumer) processTask(task *shared.ShareGroupTask) error {
	return c.processTaskAt(task, 0)
}

func (c *SharedSubscriptionConsumer) processTaskAt(task *shared.ShareGroupTask, taskOffset int) error {
	// Atomically update status: pending -> processing
	updated, err := c.store.AtomicUpdateTaskStatus(c.ctx, task.TaskID, c.shareGroup,
		shared.TaskStatusPending,
		shared.TaskStatusProcessing)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	if !updated {
		// Task was already processed by another consumer
		return nil
	}

	// Get online clients
	onlineClients, err := c.sharedStrategy().OnlineCandidates(c.ctx, domain.AssignTaskCommand{
		ShareGroup:      c.shareGroup,
		TopicFilter:     task.TopicFilter,
		PublisherClient: task.PublisherClient,
		PublishQoS:      task.PublishQoS,
		Task:            task,
	})
	if err != nil {
		_, _ = c.store.AtomicUpdateTaskStatus(c.ctx, task.TaskID, c.shareGroup,
			shared.TaskStatusProcessing,
			shared.TaskStatusPending)
		return fmt.Errorf("failed to get online clients: %w", err)
	}

	if len(onlineClients) == 0 {
		_, _ = c.store.AtomicUpdateTaskStatus(c.ctx, task.TaskID, c.shareGroup,
			shared.TaskStatusProcessing,
			shared.TaskStatusPending)
		return fmt.Errorf("no online clients available")
	}

	if err := c.assignTaskToWakeableClient(task, onlineClients, taskOffset); err != nil {
		_, _ = c.store.AtomicUpdateTaskStatus(c.ctx, task.TaskID, c.shareGroup,
			shared.TaskStatusProcessing,
			shared.TaskStatusPending)
		return err
	}

	return nil
}

func (c *SharedSubscriptionConsumer) assignTaskToWakeableClient(
	task *shared.ShareGroupTask,
	candidates []*domain.OnlineShareGroupMember,
	taskOffset int,
) error {
	remaining := orderSharedCandidates(candidates, taskOffset)
	var lastWakeErr error
	for len(remaining) > 0 {
		selected, resolved, ok := c.selectClientForTask(task, remaining, 0)
		if !ok {
			break
		}

		plan := delivery.ClientPlan{
			ClientID:            selected.ClientID,
			DeliveryQoS:         resolved.DeliveryQoS,
			SubscriptionIDsJSON: resolved.SubscriptionIDsJSON,
			WinnerNoLocal:       resolved.NoLocal,
			WinnerRAP:           resolved.RAP,
			ShareGroup:          c.shareGroup,
			SharedTaskID:        task.TaskID,
		}

		start := time.Now()
		_, inserted, err := c.taskStore.AppendClientTask(c.ctx, start, selected.ClientID, task.MessageID, plan)
		metric.RecordDeliveryEnqueueDelay("shared", plan.DeliveryQoS, sharedConsumerEnqueueResult(inserted, err), time.Since(start))
		if err == nil && !inserted {
			metric.RecordDuplicateDelivery("shared", "enqueue")
		}
		if err != nil {
			return fmt.Errorf("failed to append client task: %w", err)
		}

		wakeStart := time.Now()
		wakeMode := c.wakeMode(selected)
		wakeErr := c.wakeClient(selected, task)
		metric.RecordDeliveryWakeDelay("shared", wakeMode, sharedConsumerWakeResult(wakeErr), time.Since(wakeStart))
		if wakeErr == nil {
			return nil
		}
		lastWakeErr = wakeErr
		logger.Logger.Warn().
			Err(wakeErr).
			Str("shareGroup", c.shareGroup).
			Uint64("ownerNodeID", selected.OwnerNodeID).
			Msg("failed to wake shared subscription client")
		remaining = removeOnlineCandidate(remaining, selected.ClientID)
		remaining = prioritizeOwnerCandidates(remaining, selected.OwnerNodeID)
	}
	if lastWakeErr != nil {
		return fmt.Errorf("failed to wake any shared subscription client: %w", lastWakeErr)
	}
	return fmt.Errorf("no eligible client selected")
}

func (c *SharedSubscriptionConsumer) selectClientForTask(
	task *shared.ShareGroupTask,
	candidates []*domain.OnlineShareGroupMember,
	taskOffset int,
) (*domain.OnlineShareGroupMember, domain.ResolvedClientOptions, bool) {
	if len(candidates) == 0 {
		return nil, domain.ResolvedClientOptions{}, false
	}
	remaining := orderSharedCandidates(candidates, taskOffset)
	for len(remaining) > 0 {
		selected := remaining[0]
		if selected == nil || selected.ClientID == "" {
			remaining = remaining[1:]
			continue
		}
		resolved, ok := c.resolveSharedPlanOptions(task, selected.ClientID)
		if ok {
			return selected, resolved, true
		}
		remaining = remaining[1:]
	}
	return nil, domain.ResolvedClientOptions{}, false
}

func orderSharedCandidates(
	candidates []*domain.OnlineShareGroupMember,
	taskOffset int,
) []*domain.OnlineShareGroupMember {
	ordered := make([]*domain.OnlineShareGroupMember, len(candidates))
	copy(ordered, candidates)
	if len(ordered) == 0 {
		return ordered
	}
	offset := taskOffset % len(ordered)
	if offset < 0 {
		offset += len(ordered)
	}
	if offset == 0 {
		return ordered
	}
	return append(ordered[offset:], ordered[:offset]...)
}

func removeOnlineCandidate(
	candidates []*domain.OnlineShareGroupMember,
	clientID string,
) []*domain.OnlineShareGroupMember {
	if len(candidates) == 0 || clientID == "" {
		return candidates
	}
	out := make([]*domain.OnlineShareGroupMember, 0, len(candidates)-1)
	removed := false
	for _, candidate := range candidates {
		if !removed && candidate != nil && candidate.ClientID == clientID {
			removed = true
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func prioritizeOwnerCandidates(
	candidates []*domain.OnlineShareGroupMember,
	ownerNodeID uint64,
) []*domain.OnlineShareGroupMember {
	if len(candidates) == 0 || ownerNodeID == 0 {
		return candidates
	}
	out := make([]*domain.OnlineShareGroupMember, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate != nil && candidate.OwnerNodeID == ownerNodeID {
			out = append(out, candidate)
		}
	}
	for _, candidate := range candidates {
		if candidate == nil || candidate.OwnerNodeID == ownerNodeID {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func (c *SharedSubscriptionConsumer) resolveSharedPlanOptions(task *shared.ShareGroupTask, clientID string) (domain.ResolvedClientOptions, bool) {
	resolved, ok, err := c.sharedStrategy().ResolveClientOptions(c.ctx, domain.ResolveClientOptionsCommand{
		ShareGroup:      task.ShareGroup,
		TopicFilter:     task.TopicFilter,
		PublisherClient: task.PublisherClient,
		PublishQoS:      task.PublishQoS,
		ClientID:        clientID,
	})
	if err != nil {
		return domain.ResolvedClientOptions{}, false
	}
	return resolved, ok
}

func (c *SharedSubscriptionConsumer) wakeClient(member *domain.OnlineShareGroupMember, task *shared.ShareGroupTask) error {
	if c == nil || member == nil || member.ClientID == "" {
		return fmt.Errorf("selected shared member is empty")
	}
	if c.wakeClientFunc != nil {
		c.wakeClientFunc(member.ClientID)
		return nil
	}
	if !c.isRemoteOwner(member.OwnerNodeID) {
		return c.wakeLocalClient(member.ClientID)
	}
	if c.deliveryEvent == nil {
		return fmt.Errorf("shared remote wake event is nil")
	}
	topicFilter := ""
	if task != nil {
		topicFilter = task.TopicFilter
	}
	return c.deliveryEvent.NotifyToNode(
		c.ctx,
		member.OwnerNodeID,
		topicFilter,
		[]string{member.ClientID},
		deliveryevent.KindWake,
		nil,
		nil,
	)
}

func (c *SharedSubscriptionConsumer) wakeLocalClient(clientID string) error {
	if c.clientManager == nil {
		return fmt.Errorf("shared local wake client manager is nil")
	}
	cli, ok := c.clientManager.ReadClient(clientID)
	if !ok || cli == nil {
		return fmt.Errorf("shared local client %q is not online", clientID)
	}
	waker, ok := cli.(interface{ WakeDeliveryRunner() })
	if !ok {
		return fmt.Errorf("shared local client %q cannot wake delivery runner", clientID)
	}
	waker.WakeDeliveryRunner()
	return nil
}

func (c *SharedSubscriptionConsumer) wakeMode(member *domain.OnlineShareGroupMember) string {
	if member != nil && c.isRemoteOwner(member.OwnerNodeID) {
		return "event_remote"
	}
	return "event_local"
}

func (c *SharedSubscriptionConsumer) isRemoteOwner(ownerNodeID uint64) bool {
	return ownerNodeID != 0 && c.localNodeID != 0 && ownerNodeID != c.localNodeID
}

func sharedConsumerEnqueueResult(inserted bool, err error) string {
	if err != nil {
		return "error"
	}
	if !inserted {
		return "duplicate"
	}
	return "success"
}

func sharedConsumerWakeResult(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func (c *SharedSubscriptionConsumer) scanTimeoutTasks() {
	defer c.wg.Done()

	ticker := time.NewTicker(defaultProcessingTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.processTimeoutTasks()
		}
	}
}

func (c *SharedSubscriptionConsumer) processTimeoutTasks() {
	cutoffTime := time.Now().Add(-defaultProcessingTimeout)
	c.rollbackSvc.RequeueTimeoutTasks(c.ctx, c.shareGroup, cutoffTime)
}

func (c *SharedSubscriptionConsumer) sharedStrategy() domain.SharedDeliveryStrategy {
	if c.strategy == nil {
		c.strategy = domain.NewSharedDeliveryStrategy(c.subCenter, nil, c.clientManager)
	}
	return c.strategy
}
