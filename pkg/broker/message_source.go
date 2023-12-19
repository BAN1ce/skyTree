package broker

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/packet"
)

type MessageSource interface {
	NextMessages(ctx context.Context, n int, startMessageID string, include bool) ([]*packet.Message, int, error)
	ListenMessage(ctx context.Context) (<-chan *packet.Message, error)
	Close() error
}

type StreamSource interface {
	ListenMessage(ctx context.Context) (<-chan *packet.Message, error)
}
