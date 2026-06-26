package core

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

const brokerSessionExpiryCleanupInterval = time.Minute
const brokerSessionExpiryCleanupTimeout = 10 * time.Second

type expiredSessionDeleter interface {
	DeleteExpiredSessions(ctx context.Context, nowUnixNano int64) ([]string, error)
}

// deleteExpiredSessionsJob 定时扫描过期 session，清理与该 client 绑定的订阅和投递状态。
func (b *Broker) deleteExpiredSessionsJob() {
	ticker := time.NewTicker(brokerSessionExpiryCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !b.isRaftGroupLeader(raft.ClusterIDSessionCenter) {
				continue
			}
			b.deleteExpiredSessionsOnce(b.runtime.ctx, time.Now())
		case <-b.runtime.ctx.Done():
			return
		}
	}
}

// deleteExpiredSessionsOnce 执行一次过期 session 清理，并串联清理订阅树和投递游标状态。
func (b *Broker) deleteExpiredSessionsOnce(parent context.Context, now time.Time) {
	if b == nil || b.state.sessionCenter == nil {
		return
	}
	deleter, ok := b.state.sessionCenter.(expiredSessionDeleter)
	if !ok {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, brokerSessionExpiryCleanupTimeout)
	defer cancel()

	clientIDs, err := deleter.DeleteExpiredSessions(ctx, now.UnixNano())
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to delete expired sessions")
		return
	}
	for _, clientID := range clientIDs {
		b.cleanupExpiredClientState(ctx, clientID)
	}
}

// cleanupExpiredClientState 清理过期 session 的派生状态，避免订阅和投递游标残留。
func (b *Broker) cleanupExpiredClientState(ctx context.Context, clientID string) {
	if clientID == "" {
		return
	}
	if b.state.subCenter != nil {
		if _, err := b.state.subCenter.DeleteClient(ctx, &proto_topic.DeleteClientRequest{ClientID: clientID}); err != nil {
			logger.Logger.Warn().Err(err).Str("client", clientID).Msg("failed to delete subscriptions for expired session")
		}
	}
	if deleter, ok := b.delivery.cursorStore.(delivery.ClientStateDeleter); ok {
		if err := deleter.DeleteClientState(ctx, clientID); err != nil {
			logger.Logger.Warn().Err(err).Str("client", clientID).Msg("failed to delete delivery state for expired session")
		}
	}
}
