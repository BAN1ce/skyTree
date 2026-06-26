package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

const normalDeliveryWakeMaxAttempts = 2
const normalDeliveryWakeRetryBackoff = 25 * time.Millisecond

// appendClientDeliveryTasks appends normal subscription targets to client delivery queues and records enqueue metrics.
func (b *Broker) appendClientDeliveryTasks(
	ctx context.Context,
	now time.Time,
	topic string,
	messageID uuid.UUID,
	plans []delivery.ClientPlan,
) error {
	var appendErrs []error
	for _, plan := range plans {
		if plan.ClientID == "" {
			continue
		}
		start := time.Now()
		_, inserted, err := b.delivery.taskStore.AppendClientTask(ctx, now, plan.ClientID, messageID, plan)
		path := deliveryPathFromPlan(plan)
		metric.RecordDeliveryEnqueueDelay(path, plan.DeliveryQoS, enqueueResult(inserted, err), time.Since(start))
		if err == nil && !inserted {
			metric.RecordDuplicateDelivery(path, "enqueue")
		}
		if err != nil {
			logger.Logger.Error().Err(err).Str("client", plan.ClientID).Str("topic", topic).Msg("append delivery task failed")
			appendErrs = append(appendErrs, fmt.Errorf("client %s: %w", plan.ClientID, err))
		}
	}
	if len(appendErrs) > 0 {
		return errors.Join(appendErrs...)
	}
	return nil
}

// ownerNodeLookupCache caches client owner lookups for one delivery round to avoid duplicate session reads.
type ownerNodeLookupCache struct {
	loaded       map[string]struct{}
	nodeByClient map[string]uint64
}

func newOwnerNodeLookupCache() *ownerNodeLookupCache {
	return &ownerNodeLookupCache{
		loaded:       make(map[string]struct{}, 64),
		nodeByClient: make(map[string]uint64, 64),
	}
}

func collectUniqueClientIDs(clientIDs []string) []string {
	if len(clientIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(clientIDs))
	out := make([]string, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if clientID == "" {
			continue
		}
		if _, ok := seen[clientID]; ok {
			continue
		}
		seen[clientID] = struct{}{}
		out = append(out, clientID)
	}
	return out
}

func collectPlanClientIDs(plans []delivery.ClientPlan) []string {
	if len(plans) == 0 {
		return nil
	}
	clientIDs := make([]string, 0, len(plans))
	for _, plan := range plans {
		if plan.ClientID == "" {
			continue
		}
		clientIDs = append(clientIDs, plan.ClientID)
	}
	return clientIDs
}

// resolveOnlineOwnerNodes reads online client owner nodes in batches and returns only wakeable owners.
func (b *Broker) resolveOnlineOwnerNodes(ctx context.Context, clientIDs []string, cache *ownerNodeLookupCache) map[string]uint64 {
	out := make(map[string]uint64, len(clientIDs))
	if b == nil || b.state.sessionCenter == nil || len(clientIDs) == 0 {
		return out
	}
	if cache == nil {
		cache = newOwnerNodeLookupCache()
	}

	uniqueClientIDs := collectUniqueClientIDs(clientIDs)
	if len(uniqueClientIDs) == 0 {
		return out
	}

	pending := make([]string, 0, len(uniqueClientIDs))
	for _, clientID := range uniqueClientIDs {
		if _, ok := cache.loaded[clientID]; ok {
			continue
		}
		pending = append(pending, clientID)
	}

	if len(pending) > 0 {
		resp, err := b.state.sessionCenter.GetSessionOwners(ctx, &proto_session.ReadSessionOwnersRequest{ClientIDs: pending})
		if err == nil {
			if resp != nil {
				for _, item := range resp.GetItems() {
					if item == nil || item.GetClientID() == "" {
						continue
					}
					clientID := item.GetClientID()
					cache.loaded[clientID] = struct{}{}
					if !item.GetExist() || item.GetOwner() == nil {
						delete(cache.nodeByClient, clientID)
						continue
					}
					owner := item.GetOwner()
					nodeID := owner.GetNodeID()
					if nodeID == 0 || !owner.GetOnline() {
						delete(cache.nodeByClient, clientID)
						continue
					}
					cache.nodeByClient[clientID] = nodeID
				}
			}
			for _, clientID := range pending {
				if _, ok := cache.loaded[clientID]; ok {
					continue
				}
				cache.loaded[clientID] = struct{}{}
				delete(cache.nodeByClient, clientID)
			}
		}
	}

	for _, clientID := range uniqueClientIDs {
		nodeID, ok := cache.nodeByClient[clientID]
		if !ok {
			continue
		}
		out[clientID] = nodeID
	}
	return out
}

// wakeClientDeliveryRunners groups clients by owner node and notifies each node to wake downlink runners.
func (b *Broker) wakeClientDeliveryRunners(ctx context.Context, topic string, plans []delivery.ClientPlan) {
	if b.delivery.event == nil || b.state.sessionCenter == nil {
		return
	}

	nodeByClient := b.resolveOnlineOwnerNodes(ctx, collectPlanClientIDs(plans), nil)
	nodeClientSet := make(map[uint64]map[string]struct{}, 16)
	for _, plan := range plans {
		if plan.ClientID == "" {
			continue
		}
		nodeID, ok := nodeByClient[plan.ClientID]
		if !ok {
			continue
		}
		set, ok := nodeClientSet[nodeID]
		if !ok {
			set = make(map[string]struct{}, 8)
			nodeClientSet[nodeID] = set
		}
		set[plan.ClientID] = struct{}{}
	}

	for nodeID, set := range nodeClientSet {
		clientIDs := make([]string, 0, len(set))
		for cid := range set {
			clientIDs = append(clientIDs, cid)
		}
		b.notifyNormalClientDeliveryWake(ctx, nodeID, topic, clientIDs)
	}
}

func (b *Broker) notifyNormalClientDeliveryWake(ctx context.Context, nodeID uint64, topic string, clientIDs []string) {
	mode := b.wakeModeForNode(nodeID)
	for attempt := 1; attempt <= normalDeliveryWakeMaxAttempts; attempt++ {
		start := time.Now()
		err := b.delivery.event.NotifyToNode(ctx, nodeID, topic, clientIDs, deliveryevent.KindWake, nil, nil)
		metric.RecordDeliveryWakeDelay("normal", mode, deliveryMetricResult(err), time.Since(start))
		b.logNormalDeliveryWakeAttempt(nodeID, len(clientIDs), mode, attempt, err, time.Since(start))
		if err == nil {
			return
		}
		if attempt == normalDeliveryWakeMaxAttempts || !sleepBeforeNormalDeliveryWakeRetry(ctx) {
			return
		}
	}
}

func (b *Broker) logNormalDeliveryWakeAttempt(
	nodeID uint64,
	clientCount int,
	mode string,
	attempt int,
	err error,
	elapsed time.Duration,
) {
	event := logger.Logger.Debug()
	if err != nil {
		event = logger.Logger.Warn().Err(err)
	}
	event.
		Uint64("node_id", nodeID).
		Int("client_count", clientCount).
		Str("mode", mode).
		Int("attempt", attempt).
		Int("max_attempts", normalDeliveryWakeMaxAttempts).
		Dur("elapsed", elapsed).
		Msg("normal delivery wake notify result")
}

func sleepBeforeNormalDeliveryWakeRetry(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(normalDeliveryWakeRetryBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func deliveryPathFromPlan(plan delivery.ClientPlan) string {
	if plan.ShareGroup != "" || plan.SharedTaskID != uuid.Nil {
		return "shared"
	}
	return "normal"
}

func enqueueResult(inserted bool, err error) string {
	if err != nil {
		return "error"
	}
	if !inserted {
		return "duplicate"
	}
	return "success"
}

func deliveryMetricResult(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func (b *Broker) wakeModeForNode(nodeID uint64) string {
	if b != nil && b.cluster.nodeMeta != nil && b.cluster.nodeMeta.LocalNodeID == nodeID {
		return "event_local"
	}
	return "event_remote"
}
