package broker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/BAN1ce/skyTree/pkg/util"
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
}

// Encode publish packet to bytes
func Encode(version SerialVersion, publish *packet.PublishMessage, buf *bytes.Buffer) error {
	if serializer, ok := storeSerializerFactory[version]; ok {
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

type SerialVersion = byte

const (
	SerialVersion1 = SerialVersion(iota + 1)
)

var (
	storeSerializerFactory = map[byte]StoreSerializer{
		SerialVersion1: serializerVersion1{},
	}
)

type StoreSerializer interface {
	Encode(publish *packet.PublishMessage, buf *bytes.Buffer) error
	Decode(rawData []byte) (*packet.PublishMessage, error)
}

type serializerVersion1 struct {
}

func (serializerVersion1) Encode(pubMessage *packet.PublishMessage, buf *bytes.Buffer) error {
	var (
		expiredTime  int64
		publish      = pubMessage.PublishPacket
		headerLength int64
	)
	if publish.Properties.MessageExpiry != nil {
		expiredTime = int64(*publish.Properties.MessageExpiry)
	}
	buf.WriteByte(SerialVersion1)
	if err := util.WriteUint64(uint64(expiredTime), buf); err != nil {
		return err
	}
	if err := util.WriteUint32(uint32(time.Now().Unix()), buf); err != nil {
		return err
	}
	headerLength = int64(buf.Len())
	if n, err := publish.WriteTo(buf); err != nil {
		return err
	} else if n+headerLength != int64(buf.Len()) {
		return fmt.Errorf("write to buffer error, expect %d, got %d", buf.Len(), n)
	}
	return nil
}

// Decode bytes to publish packet
func (serializerVersion1) Decode(rawData []byte) (*packet.PublishMessage, error) {
	var (
		bf = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	if len(rawData) <= 4 {
		return nil, errs.ErrStoreMessageLength
	}
	bf.Write(rawData)
	expiredTimestamp, err := util.ReadUint64(bf)
	if err != nil {
		return nil, err
	}
	createTime, err := util.ReadUint32(bf)
	if err != nil {
		return nil, errs.ErrStoreReadCreateTime
	}
	if ctl, err := packets.ReadPacket(bf); err != nil {
		return nil, err
	} else {
		return &packet.PublishMessage{
			MessageID:     "",
			PublishPacket: ctl.Content.(*packets.Publish),
			PubRelPacket:  nil,
			PubReceived:   false,
			FromSession:   false,
			TimeStamp:     int64(createTime),
			ExpiredTime:   int64(expiredTimestamp),
		}, nil
	}
}
