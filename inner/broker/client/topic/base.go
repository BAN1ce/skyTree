package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"go.uber.org/zap"
)

type meta struct {
	topic           string
	windowSize      int
	latestMessageID string
}

func FillUnfinishedMessage(ctx context.Context, message []*packet.Message, source broker.MessageSource) []*packet.Message {
	for i := 0; i < len(message); i++ {
		msg := message[i]
		if m, n, err := source.NextMessages(ctx, 1, msg.MessageID, true); err != nil || n == 0 {
			logger.Logger.Warn("fill unfinished message error", zap.Error(err))
			continue
		} else {
			msg.PublishPacket = m[0].PublishPacket
		}
	}
	logger.Logger.Debug("fill unfinished message", zap.Int("message", len(message)))
	return message
}
