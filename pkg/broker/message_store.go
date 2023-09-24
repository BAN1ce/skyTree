package broker

import (
	"bytes"
	"context"
	"github.com/BAN1ce/skyTree/pkg/broker/message/serializer"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"time"
)

type PublishElement interface {
	GetTopic() string
	GetResponseTopic() string
	GetQos() int
	GetContentType() string
}

type Store interface {
	TopicMessageStore
}

type TopicStoreInfo interface {
	GetTopicMessageTotalCount(ctx context.Context, topic string) (int64, error)
	DeleteTopicMessages(ctx context.Context, topic string) error
}

type TopicMessageStore interface {
	ReadFromTimestamp(ctx context.Context, topic string, timestamp time.Time, limit int) ([]packet.PublishMessage, error)
	ReadTopicMessagesByID(ctx context.Context, topic, id string, limit int, include bool) ([]packet.PublishMessage, error)
	CreatePacket(topic string, value []byte) (id string, err error)
	DeleteTopicMessageID(ctx context.Context, topic, messageID string) error
}

// Encode publish packet to bytes
func Encode(version serializer.SerialVersion, publish *packet.PublishMessage, buf *bytes.Buffer) error {
	if serializer, ok := storeSerializerFactory[version]; ok {
		buf.WriteByte(version)
		return serializer.Encode(publish, buf)
	} else {
		return errs.ErrStoreVersionInvalid
	}
}

// Decode bytes to publish packet
func Decode(rawData []byte) (*packet.PublishMessage, error) {
	if len(rawData) == 0 {
		return nil, errs.ErrStoreMessageLength
	}
	version := rawData[0]
	if serializer, ok := storeSerializerFactory[version]; ok {
		return serializer.Decode(rawData[1:])
	} else {
		return nil, errs.ErrStoreVersionInvalid
	}
}

var (
	storeSerializerFactory = map[byte]StoreSerializer{
		serializer.ProtoBufVersion: &serializer.ProtoBufSerializer{},
	}
)

type StoreSerializer interface {
	Encode(publish *packet.PublishMessage, buf *bytes.Buffer) error
	Decode(rawData []byte) (*packet.PublishMessage, error)
}
