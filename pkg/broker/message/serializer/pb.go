package serializer

import (
	"bytes"
	"fmt"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
	"google.golang.org/protobuf/proto"
)

const ProtoBufVersion = SerialVersion(iota + 1)

type ProtoBufSerializer struct {
}

func (p *ProtoBufSerializer) Encode(pub *packet.Message, buf *bytes.Buffer) error {
	var (
		pubBuf       = pool.ByteBufferPool.Get()
		pubRelBuf    = pool.ByteBufferPool.Get()
		protoMessage = packet.StorePublishMessage{
			MessageID:   pub.MessageID,
			PubReceived: pub.IsPubReceived(),
			FromSession: pub.IsFromSession(),
			TimeStamp:   pub.Timestamp,
			ExpiredTime: pub.ExpiredTime,
			Will:        pub.IsWill(),
			ClientID:    pub.ClientID,
		}
	)
	defer func() {
		pool.ByteBufferPool.Put(pubBuf)
		pool.ByteBufferPool.Put(pubRelBuf)
	}()
	if pub.PublishPacket != nil {
		if _, err := pub.PublishPacket.WriteTo(pubBuf); err != nil {
			return err
		}
	}
	if pub.PubRelPacket != nil {
		if _, err := pub.PubRelPacket.WriteTo(pubRelBuf); err != nil {
			return err
		}
	}

	protoMessage.PubRelPacket = pubRelBuf.Bytes()
	protoMessage.PublishPacket = pubBuf.Bytes()
	pbBody, err := proto.Marshal(&protoMessage)
	if err != nil {
		return err
	}
	if n, err := buf.Write(pbBody); err != nil {
		return err
	} else if n != len(pbBody) {
		return fmt.Errorf("write to buffer error, expect %d, got %d", len(pbBody), n)
	}
	return nil

}

func (p *ProtoBufSerializer) Decode(rawData []byte) (*packet.Message, error) {
	var (
		protoMessage = packet.StorePublishMessage{}
		bf           = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	if err := proto.Unmarshal(rawData, &protoMessage); err != nil {
		return nil, err
	}
	bf.Write(protoMessage.PublishPacket)

	if ctl, err := packets.ReadPacket(bf); err != nil {
		return nil, err
	} else {
		return &packet.Message{
			ClientID:      protoMessage.ClientID,
			MessageID:     protoMessage.MessageID,
			PublishPacket: ctl.Content.(*packets.Publish),
			PubReceived:   protoMessage.PubReceived,
			Timestamp:     protoMessage.TimeStamp,
			ExpiredTime:   protoMessage.ExpiredTime,
			Will:          protoMessage.Will,
		}, nil
	}
}
