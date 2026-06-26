package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/consumer"
	"github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/domain"
	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription/leader"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription/selector"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/google/uuid"
)

const (
	defaultLeaderTTL = 10 * time.Second
	rollbackDelay    = 500 * time.Millisecond
)

// SharedSubscriptionManager manages shared subscription lifecycle and consumers
type SharedSubscriptionManager struct {
	store         store.SharedSubscriptionStore
	sessionCenter session.Center
	clientManager consumer.ClientManagerInterface
	selector      selector.ShareGroupSelector
	strategy      domain.SharedDeliveryStrategy
	taskStore     delivery.TaskStore
	cursorStore   delivery.CursorStore
	subCenter     subscription.Center
	rollbackSvc   *domain.RollbackService
	deliveryEvent delivery_notify.ClientDeliveryEvent

	consumers    map[string]*consumer.SharedSubscriptionConsumer // shareGroup -> consumer
	consumersMux sync.RWMutex

	clientStates   map[string]*clientSharedState
	clientStateMux sync.RWMutex

	nodeID         uint64
	nodeMeta       *cluster.NodeMeta
	leaderElection leader.LeaderElection

	lifecycleMu sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	stopped     bool
	wg          sync.WaitGroup
}

type clientSharedState struct {
	groups map[string]*shareGroupState
}

type shareGroupState struct {
	topicFilters map[string]struct{}
}

// NewSharedSubscriptionManager creates a new shared subscription manager
func NewSharedSubscriptionManager(
	store store.SharedSubscriptionStore,
	sessionCenter session.Center,
	clientManager consumer.ClientManagerInterface,
	selector selector.ShareGroupSelector,
	taskStore delivery.TaskStore,
	cursorStore delivery.CursorStore,
	subCenter subscription.Center,
	nodeID uint64,
	nodeMeta *cluster.NodeMeta,
	leaderElection leader.LeaderElection,
	deliveryEvents ...delivery_notify.ClientDeliveryEvent,
) *SharedSubscriptionManager {
	var deliveryEvent delivery_notify.ClientDeliveryEvent
	if len(deliveryEvents) > 0 {
		deliveryEvent = deliveryEvents[0]
	}
	m := &SharedSubscriptionManager{
		store:          store,
		sessionCenter:  sessionCenter,
		clientManager:  clientManager,
		selector:       selector,
		strategy:       domain.NewSharedDeliveryStrategy(subCenter, sessionCenter, clientManager),
		taskStore:      taskStore,
		cursorStore:    cursorStore,
		subCenter:      subCenter,
		consumers:      make(map[string]*consumer.SharedSubscriptionConsumer),
		clientStates:   make(map[string]*clientSharedState),
		nodeID:         nodeID,
		nodeMeta:       nodeMeta,
		leaderElection: leaderElection,
		deliveryEvent:  deliveryEvent,
	}
	m.rollbackSvc = domain.NewRollbackService(store, cursorStore, m.wakeConsumer)
	return m
}

// Start starts the manager
func (m *SharedSubscriptionManager) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.lifecycleMu.Lock()
	if m.cancel != nil {
		m.lifecycleMu.Unlock()
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.stopped = false
	startLeaderRenewal := m.leaderElection != nil
	if startLeaderRenewal {
		m.wg.Add(1)
	}
	m.lifecycleMu.Unlock()

	if startLeaderRenewal {
		go m.leaderRenewalLoop()
	}

	return nil
}

// Stop stops the manager
func (m *SharedSubscriptionManager) Stop() {
	if m == nil {
		return
	}
	m.lifecycleMu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.stopped = true
	m.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}

	// Stop all consumers
	m.consumersMux.Lock()
	for shareGroup, c := range m.consumers {
		c.Stop()
		delete(m.consumers, shareGroup)
	}
	m.consumersMux.Unlock()

	m.wg.Wait()
}

func (m *SharedSubscriptionManager) runtimeContext(fallback context.Context) context.Context {
	if m == nil {
		if fallback != nil {
			return fallback
		}
		return context.Background()
	}
	m.lifecycleMu.RLock()
	ctx := m.ctx
	m.lifecycleMu.RUnlock()
	if ctx != nil {
		return ctx
	}
	if fallback != nil {
		return fallback
	}
	return context.Background()
}

func (m *SharedSubscriptionManager) runManaged(fn func(context.Context)) {
	if m == nil || fn == nil {
		return
	}
	m.lifecycleMu.RLock()
	if m.stopped {
		m.lifecycleMu.RUnlock()
		return
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	m.wg.Add(1)
	m.lifecycleMu.RUnlock()

	go func() {
		defer m.wg.Done()
		fn(ctx)
	}()
}

// OnClientOnline handles client online event
func (m *SharedSubscriptionManager) OnClientOnline(ctx context.Context, clientID string, shareGroup string, topicFilter string) error {
	if shareGroup == "" || topicFilter == "" {
		return fmt.Errorf("invalid shareGroup or topicFilter")
	}
	m.rememberClientSharedTopicFilter(clientID, shareGroup, topicFilter)

	// Member information is automatically managed by SubCenter through subscription operations
	// No need to register separately

	// Ensure consumer is running
	if err := m.ensureConsumer(ctx, shareGroup, topicFilter); err != nil {
		return fmt.Errorf("failed to ensure consumer: %w", err)
	}

	return nil
}

// OnClientOffline handles client offline event
func (m *SharedSubscriptionManager) OnClientOffline(ctx context.Context, clientID string) error {
	// Get all share groups for this client
	shareGroups, err := m.getClientShareGroups(ctx, clientID)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("clientID", clientID).Msg("failed to get client share groups")
	}

	// Process each share group
	for _, shareGroup := range shareGroups {
		// Delay rollback to allow quick reconnect
		shareGroup := shareGroup
		m.runManaged(func(runCtx context.Context) {
			m.delayedRollbackUnackedTasks(runCtx, clientID, shareGroup)
		})

		// Member information is automatically managed by SubCenter through unsubscription operations
		// No need to unregister separately
	}

	return nil
}

// OnClientUnsubscribe handles one shared-subscription unsubscribe without treating the whole client as offline.
func (m *SharedSubscriptionManager) OnClientUnsubscribe(ctx context.Context, clientID string, shareGroup string, topicFilter string) error {
	if shareGroup == "" || topicFilter == "" {
		return fmt.Errorf("invalid shareGroup or topicFilter")
	}
	groupRemoved := m.forgetClientSharedTopicFilter(clientID, shareGroup, topicFilter)
	if groupRemoved && m.store != nil && m.cursorStore != nil {
		m.runManaged(func(runCtx context.Context) {
			m.rollbackSvc.RollbackClientTasks(runCtx, domain.RollbackCommand{
				ClientID:   clientID,
				ShareGroup: shareGroup,
				Reason:     "unsubscribe",
			})
		})
	}
	return nil
}

func (m *SharedSubscriptionManager) MarkTaskProcessed(ctx context.Context, shareGroup string, taskID uuid.UUID) error {
	if m == nil || m.store == nil {
		return nil
	}
	if shareGroup == "" || taskID == uuid.Nil {
		return nil
	}
	return m.store.MarkShareGroupTaskProcessed(ctx, taskID, shareGroup)
}

func (m *SharedSubscriptionManager) getClientShareGroups(ctx context.Context, clientID string) ([]string, error) {
	_ = ctx
	m.clientStateMux.RLock()
	defer m.clientStateMux.RUnlock()
	state := m.clientStates[clientID]
	if state == nil {
		return nil, nil
	}
	result := make([]string, 0, len(state.groups))
	for sg := range state.groups {
		result = append(result, sg)
	}

	return result, nil
}

func (m *SharedSubscriptionManager) rememberClientShareGroup(clientID, shareGroup string) {
	if m == nil || clientID == "" || shareGroup == "" {
		return
	}
	m.clientStateMux.Lock()
	defer m.clientStateMux.Unlock()
	state := m.ensureClientStateLocked(clientID)
	state.ensureGroup(shareGroup)
}

func (m *SharedSubscriptionManager) rememberClientSharedTopicFilter(clientID, shareGroup, topicFilter string) {
	if m == nil || clientID == "" || shareGroup == "" || topicFilter == "" {
		return
	}
	m.clientStateMux.Lock()
	defer m.clientStateMux.Unlock()
	state := m.ensureClientStateLocked(clientID)
	group := state.ensureGroup(shareGroup)
	group.topicFilters[topicFilter] = struct{}{}
}

func (m *SharedSubscriptionManager) forgetClientShareGroup(clientID, shareGroup string) {
	if m == nil || clientID == "" || shareGroup == "" {
		return
	}
	m.clientStateMux.Lock()
	defer m.clientStateMux.Unlock()
	m.forgetClientShareGroupLocked(clientID, shareGroup)
}

func (m *SharedSubscriptionManager) forgetClientSharedTopicFilter(clientID, shareGroup, topicFilter string) bool {
	if m == nil || clientID == "" || shareGroup == "" || topicFilter == "" {
		return false
	}
	m.clientStateMux.Lock()
	defer m.clientStateMux.Unlock()

	state := m.clientStates[clientID]
	if state == nil {
		return false
	}
	group := state.groups[shareGroup]
	if group == nil {
		return false
	}
	delete(group.topicFilters, topicFilter)
	if len(group.topicFilters) > 0 {
		return false
	}

	return m.forgetClientShareGroupLocked(clientID, shareGroup)
}

func (m *SharedSubscriptionManager) forgetClientShareGroupLocked(clientID, shareGroup string) bool {
	state := m.clientStates[clientID]
	if state == nil {
		return false
	}
	delete(state.groups, shareGroup)
	if len(state.groups) == 0 {
		delete(m.clientStates, clientID)
	}
	return true
}

func (m *SharedSubscriptionManager) ensureClientStateLocked(clientID string) *clientSharedState {
	if m.clientStates == nil {
		m.clientStates = make(map[string]*clientSharedState)
	}
	state := m.clientStates[clientID]
	if state == nil {
		state = &clientSharedState{groups: make(map[string]*shareGroupState)}
		m.clientStates[clientID] = state
	}
	return state
}

func (s *clientSharedState) ensureGroup(shareGroup string) *shareGroupState {
	if s.groups == nil {
		s.groups = make(map[string]*shareGroupState)
	}
	group := s.groups[shareGroup]
	if group == nil {
		group = &shareGroupState{topicFilters: make(map[string]struct{})}
		s.groups[shareGroup] = group
	}
	return group
}

func (m *SharedSubscriptionManager) delayedRollbackUnackedTasks(ctx context.Context, clientID string, shareGroup string) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Wait for delay to allow quick reconnect
	timer := time.NewTimer(rollbackDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	// Check if client is back online
	if m.clientManager != nil {
		if cli, ok := m.clientManager.ReadClient(clientID); ok && cli != nil {
			// Client is back online, skip rollback
			logger.Logger.Debug().Str("clientID", clientID).Str("shareGroup", shareGroup).Msg("client reconnected, skipping rollback")
			return
		}
	}

	m.rollbackSvc.RollbackClientTasks(ctx, domain.RollbackCommand{
		ClientID:   clientID,
		ShareGroup: shareGroup,
		Reason:     "offline",
	})
}

func (m *SharedSubscriptionManager) rollbackUnackedTasks(ctx context.Context, clientID string, shareGroup string) {
	if m == nil || m.rollbackSvc == nil {
		return
	}
	m.rollbackSvc.RollbackClientTasks(ctx, domain.RollbackCommand{
		ClientID:   clientID,
		ShareGroup: shareGroup,
		Reason:     "manual",
	})
}

type deliveryCursor struct {
	LastTS     time.Time
	LastTaskID uuid.UUID
}

func (m *SharedSubscriptionManager) getDeliveryCursor(ctx context.Context, clientID string) (*deliveryCursor, error) {
	if m == nil || m.cursorStore == nil {
		return &deliveryCursor{LastTS: time.Time{}, LastTaskID: uuid.Nil}, nil
	}
	cursor, err := m.cursorStore.ReadCursor(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if cursor == nil {
		return &deliveryCursor{LastTS: time.Time{}, LastTaskID: uuid.Nil}, nil
	}
	return &deliveryCursor{LastTS: cursor.LastTS, LastTaskID: cursor.LastTaskID}, nil
}

func (m *SharedSubscriptionManager) ensureConsumer(ctx context.Context, shareGroup string, topicFilter string) error {
	m.consumersMux.Lock()
	defer m.consumersMux.Unlock()

	// Check if consumer already exists
	if _, exists := m.consumers[shareGroup]; exists {
		return nil
	}

	// Try to acquire leadership
	if m.leaderElection != nil {
		acquired, err := m.leaderElection.TryAcquireLeadership(ctx, shareGroup, m.nodeID, defaultLeaderTTL)
		if err != nil {
			return fmt.Errorf("failed to acquire leadership: %w", err)
		}
		if !acquired {
			// Not the leader, watch for leadership changes
			m.runManaged(func(runCtx context.Context) {
				m.watchLeadership(runCtx, shareGroup, topicFilter)
			})
			return nil
		}
	}

	// Create and start consumer
	c := consumer.NewSharedSubscriptionConsumer(
		shareGroup,
		topicFilter,
		m.store,
		m.taskStore,
		m.clientManager,
		m.selector,
		m.subCenter,
		m.strategy,
		m.rollbackSvc,
		m.nodeID,
		m.deliveryEvent,
	)
	m.consumers[shareGroup] = c
	c.Run(m.runtimeContext(ctx))

	return nil
}

func (m *SharedSubscriptionManager) watchLeadership(ctx context.Context, shareGroup string, topicFilter string) {
	if m.leaderElection == nil {
		return
	}

	_ = m.leaderElection.WatchLeadership(ctx, shareGroup, func(isLeader bool) {
		if ctx.Err() != nil {
			return
		}
		if isLeader {
			// Become leader, start consumer
			m.ensureConsumer(ctx, shareGroup, topicFilter)
		} else {
			// Lost leadership, stop consumer
			m.stopConsumer(shareGroup)
		}
	})
}

func (m *SharedSubscriptionManager) stopConsumer(shareGroup string) {
	m.consumersMux.Lock()
	defer m.consumersMux.Unlock()

	if c, exists := m.consumers[shareGroup]; exists {
		c.Stop()
		delete(m.consumers, shareGroup)
	}
}

func (m *SharedSubscriptionManager) wakeConsumer(shareGroup string) {
	m.consumersMux.RLock()
	c, exists := m.consumers[shareGroup]
	m.consumersMux.RUnlock()

	if exists && c != nil {
		c.Wake()
	}
}

// WakeShareGroup wakes the consumer for a share group
func (m *SharedSubscriptionManager) WakeShareGroup(shareGroup string) {
	m.wakeConsumer(shareGroup)
}

func (m *SharedSubscriptionManager) leaderRenewalLoop() {
	defer m.wg.Done()

	if m.leaderElection == nil {
		return
	}
	ctx := m.runtimeContext(context.Background())

	ticker := time.NewTicker(defaultLeaderTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.renewLeaderships(ctx)
		}
	}
}

func (m *SharedSubscriptionManager) renewLeaderships(ctx context.Context) {
	m.consumersMux.RLock()
	shareGroups := make([]string, 0, len(m.consumers))
	for sg := range m.consumers {
		shareGroups = append(shareGroups, sg)
	}
	m.consumersMux.RUnlock()

	for _, shareGroup := range shareGroups {
		if ctx.Err() != nil {
			return
		}
		if err := m.leaderElection.RenewLeadership(ctx, shareGroup, m.nodeID, defaultLeaderTTL); err != nil {
			logger.Logger.Warn().Err(err).Str("shareGroup", shareGroup).Msg("failed to renew leadership")
			// Leadership lost, stop consumer
			m.stopConsumer(shareGroup)
		}
	}
}
