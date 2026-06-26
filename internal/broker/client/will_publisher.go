package client

import (
	"context"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

type willMessagePublisher struct {
	ch         chan<- *brokerpublish.Message
	clientID   string
	ownerToken string
}

func (c *Client) willPublisher() willMessagePublisher {
	if c == nil || c.component == nil {
		return willMessagePublisher{}
	}
	return willMessagePublisher{
		ch:         c.component.notifyWillMessageChan,
		clientID:   c.getID(),
		ownerToken: c.getOwnerToken(),
	}
}

func (p willMessagePublisher) publish(ctx context.Context, publishContent *packets.Publish, willDelay time.Duration) {
	if p.ch == nil || publishContent == nil {
		return
	}
	msg := &brokerpublish.Message{
		Publish:      publishContent,
		SendClientID: p.clientID,
		OwnerToken:   p.ownerToken,
		Will:         true,
		WillDelay:    willDelay,
	}
	select {
	case p.ch <- msg:
	case <-ctx.Done():
	}
}
