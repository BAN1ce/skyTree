package client

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

func (c *Client) removeOutgoingUnfinishedFromSession(messageID uuid.UUID) {
	if c == nil || messageID == uuid.Nil {
		return
	}
	if c.component == nil || c.component.sessionCenter == nil {
		return
	}
	clientID := c.getID()
	if clientID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	target := messageID.String()
	if err := c.component.sessionCenter.RemoveOutgoingUnfinished(ctx, &proto_session.RemoveOutgoingUnfinishedRequest{
		ClientID:    clientID,
		MessageID:   target,
		NowUnixNano: time.Now().UnixNano(),
	}); err != nil {
		logger.Logger.Debug().Err(err).Str("client", c.metaString()).Str("message_id", target).Msg("failed to clear outgoing unfinished from session")
	}
}
