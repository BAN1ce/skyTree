package core

import (
	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/google/uuid"
)

// CallRetry 在发布重试任务触发时定位在线 client，并将消息交回 client 侧重发。
func (b *Broker) CallRetry(task *retry.Task) error {
	if b == nil || task == nil || task.Data == nil {
		logger.Logger.Warn().Msg("skip publish retry: invalid retry task")
		return nil
	}
	clientID := task.Data.SendClientID
	if clientID == "" {
		clientID = task.ClientID
	}
	if clientID == "" {
		logger.Logger.Warn().Str("retry_key", task.Key).Msg("skip publish retry: empty client id")
		return nil
	}
	if b.clients.manager == nil {
		logger.Logger.Warn().Str("retry_key", task.Key).Str("client_id", clientID).Msg("skip publish retry: client manager is nil")
		return nil
	}

	b.network.mux.RLock()
	c, ok := b.clients.manager.ReadClient(clientID)
	b.network.mux.RUnlock()

	if ok {
		return c.RetrySend(task.Data)
	}
	return nil
}

// CallTimeout handles publish retry timeout for the current connection while preserving persistent session state.
func (b *Broker) CallTimeout(task *retry.Task) error {
	metric.RecordPublishRetryAction(metric.PublishRetryActionTimeout)
	if b == nil || task == nil || task.Data == nil {
		return nil
	}

	message := task.Data
	clientID := message.SendClientID
	if clientID == "" {
		clientID = task.ClientID
	}
	messageID := message.MessageID
	retryKey := task.Key
	if retryKey == "" && message.RetryInfo != nil {
		retryKey = message.RetryInfo.Key
	}

	logEvent := logger.Logger.Warn().
		Str("retry_key", retryKey).
		Str("client_id", clientID)
	if messageID != uuid.Nil {
		logEvent = logEvent.Str("message_id", messageID.String())
	}
	logEvent.Msg("publish retry timeout reached")

	b.disconnectClientForPublishRetryTimeout(clientID, retryKey)
	return nil
}

func (b *Broker) disconnectClientForPublishRetryTimeout(clientID, retryKey string) {
	if b == nil || b.clients.manager == nil || clientID == "" {
		return
	}

	b.network.mux.RLock()
	c, ok := b.clients.manager.ReadClient(clientID)
	b.network.mux.RUnlock()
	if !ok {
		return
	}

	err := c.CloseWithDisconnectKeepWill(
		brokerclient.DisconnectForQuotaExceeded("publish retry timeout reached without ACK"),
	)
	if err != nil {
		logger.Logger.Debug().
			Err(err).
			Str("retry_key", retryKey).
			Str("client_id", clientID).
			Msg("publish retry timeout disconnect failed")
	}
}
