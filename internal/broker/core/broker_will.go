package core

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
)

const brokerWillDrainTimeout = 10 * time.Second
const brokerWillProcessTimeout = 30 * time.Second

// listenWillMessage 消费 client 生命周期中产生的遗嘱消息，并在关闭时尽量排空队列。
func (b *Broker) listenWillMessage() {
	for {
		select {
		case willMsg, ok := <-b.will.messageChan:
			if !ok {
				return
			}
			b.processWillMessage(b.runtime.ctx, willMsg)

		case <-b.runtime.ctx.Done():
			b.drainWillMessages(brokerWillDrainTimeout)
			return
		}
	}
}

// processWillMessage 按普通 publish 路径处理遗嘱消息，成功后清理 session 中的 will 状态。
func (b *Broker) processWillMessage(parent context.Context, willMsg *brokerpublish.Message) {
	if willMsg == nil {
		return
	}
	publish := willMsg.GetPublish()
	if publish == nil {
		logger.Logger.Error().Msg("will message publish is nil")
		return
	}

	base := parent
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, brokerWillProcessTimeout)
	defer cancel()

	if b.pluginSet.hooks != nil {
		if err := b.pluginSet.hooks.DoReceivedPublish(ctx, willMsg.SendClientID, publish); err != nil {
			logger.Logger.Warn().Err(err).Str("client", willMsg.SendClientID).Str("topic", publish.Topic).Msg("will publish denied by plugin")
			return
		}
	}
	if err := b.routePublishClientMode(ctx, willMsg); err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client", willMsg.SendClientID).
			Str("topic", publish.Topic).
			Msg("failed to process will publish")
		return
	}
	b.clearPublishedWillFromSession(ctx, willMsg)
}

// drainWillMessages 在关闭窗口内处理队列中的遗嘱消息，避免优雅关闭时直接丢弃。
func (b *Broker) drainWillMessages(timeout time.Duration) {
	if b == nil || timeout <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			if pending := len(b.will.messageChan); pending > 0 {
				logger.Logger.Warn().
					Int("pending", pending).
					Dur("timeout", timeout).
					Msg("broker shutdown will-drain timeout reached")
			}
			return
		case willMsg, ok := <-b.will.messageChan:
			if !ok {
				return
			}
			b.processWillMessage(ctx, willMsg)
		default:
			return
		}
	}
}
