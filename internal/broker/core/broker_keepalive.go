package core

import (
	"time"

	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
)

const defaultBrokerKeepAliveScanInterval = time.Second

// deleteUnAliveClientTimeJob periodically closes clients that exceed keepalive timeout while preserving Will semantics.
func (b *Broker) deleteUnAliveClientTimeJob() {
	tk := time.NewTicker(b.keepAliveScanInterval())
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			b.deleteUnAliveClient()
		case <-b.runtime.ctx.Done():
			return
		}
	}
}

func (b *Broker) keepAliveScanInterval() time.Duration {
	if b == nil || b.config.broker.KeepAliveScanInterval <= 0 {
		return defaultBrokerKeepAliveScanInterval
	}
	return b.config.broker.KeepAliveScanInterval
}

// deleteUnAliveClient scans the keepalive tracker and actively closes clients that are truly idle.
func (b *Broker) deleteUnAliveClient() {
	if b == nil || b.clients.manager == nil || b.clients.keepAliveTracker == nil {
		return
	}
	now := time.Now()
	for _, expired := range b.clients.keepAliveTracker.ScanExpired(now) {
		c, ok := b.clients.manager.ReadClient(expired.ClientID)
		if !ok {
			continue
		}
		if c == nil || c.GetOwnerToken() != expired.OwnerToken {
			metric.RecordOwnerTokenConflict("session", "ignore")
			continue
		}
		if !c.KeepAliveExpired(now) {
			b.clients.keepAliveTracker.Update(c.GetID(), c.GetOwnerToken(), now.Add(-c.IdleDurationSince(now)), c.GetKeepAliveTime())
			continue
		}
		keepAlive := c.GetKeepAliveTime()
		idle := c.IdleDurationSince(now)
		logger.Logger.Info().
			Str("client", c.GetID()).
			Dur("keep_alive", keepAlive).
			Dur("idle", idle).
			Msg("closing client after keepalive timeout")
		// MQTT5 sections 3.1.2.10 and 4.13.2 require DISCONNECT(0x8D)
		// when the server closes a connection for keepalive timeout; Will publication remains enabled.
		disc := brokerclient.DisconnectForKeepAliveTimeout(idle, keepAlive)
		if err := c.CloseWithDisconnectKeepWill(disc); err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.GetID()).Msg("failed to close keepalive-timeout client")
		}
		b.clients.manager.DeleteClient(c)
	}
}
