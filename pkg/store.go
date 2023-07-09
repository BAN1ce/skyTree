package pkg

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"time"
)

type PublishElement interface {
	GetTopic() string
	GetResponseTopic() string
	GetQos() int
	GetContentType() string
}

type Store interface {
	ClientMessageStore
	PublishedStore
}

type TopicStoreInfo interface {
	GetTopicMessageTotalCount(ctx context.Context, topic string) (int64, error)
	DeleteTopicMessages(ctx context.Context, topic string) error
}

type ClientMessageStore interface {
	ReadFromTimestamp(ctx context.Context, topic string, timestamp time.Time, limit int) ([]packet.PublishMessage, error)
	ReadTopicMessagesByID(ctx context.Context, topic, id string, limit int, include bool) ([]packet.PublishMessage, error)
}

type PublishedStore interface {
	CreatePacket(topic string, value []byte) (id string, err error)
}

// Encode publish packet to bytes
func Encode(publish *packets.Publish) ([]byte, error) {
	var (
		bf = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	if _, err := publish.WriteTo(bf); err != nil {
		return nil, err
	} else {
		return bf.Bytes(), nil
	}
}

// Decode bytes to publish packet
func Decode(rawData []byte) (*packets.Publish, error) {
	var (
		bf = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	bf.Write(rawData)
	if ctl, err := packets.ReadPacket(bf); err != nil {
		return nil, err
	} else {
		return ctl.Content.(*packets.Publish), nil
	}
}
