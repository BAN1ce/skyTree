package pkg

import (
	"context"
)

type Store interface {
	ClientMessageStore
	PublishedStore
}

type ClientMessageStore interface {
	ReadTopicMessageByID(ctx context.Context, topic, id string, limit int) []Message
}

type PublishedStore interface {
	CreatePacket(topic string, value []byte) (id string, err error)
}
