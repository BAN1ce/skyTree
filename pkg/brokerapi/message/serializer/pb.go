package serializer

import (
	"bytes"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/publish/publishpb"
	"github.com/BAN1ce/skyTree/pkg/bufferpool"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

var (
	Serializer = &ProtoBufSerializer{}
)

type ProtoBufSerializer struct {
}

func (p *ProtoBufSerializer) Encode(publish *brokerpublish.Message) ([]byte, error) {
	var buffer = bufferpool.ByteBufferPool.Get()
	defer bufferpool.ByteBufferPool.Put(buffer)
	var protoMessage, err = PublishMessageToProtoMessage(publish, buffer)

	if err != nil {
		return nil, err
	}

	return proto.Marshal(protoMessage)

}

func (p *ProtoBufSerializer) BatchEncode(publish []*brokerpublish.Message) ([]byte, error) {
	var (
		batchStoreMessage = new(publishpb.BatchStorePublishMessage)
		backPool          []*bytes.Buffer
	)
	defer func() {
		for _, v := range backPool {
			bufferpool.ByteBufferPool.Put(v)
		}
	}()

	for _, v := range publish {
		var buffer = bufferpool.ByteBufferPool.Get()
		backPool = append(backPool, buffer)
		protoMessage, err := PublishMessageToProtoMessage(v, buffer)
		if err != nil {
			return nil, err
		}
		batchStoreMessage.Message = append(batchStoreMessage.Message, protoMessage)
	}

	return proto.Marshal(batchStoreMessage)
}

func (p *ProtoBufSerializer) BatchDecode(rawData []byte) ([]*brokerpublish.Message, error) {
	var (
		batchStoreMessage = new(publishpb.BatchStorePublishMessage)
		result            []*brokerpublish.Message
	)

	if err := proto.Unmarshal(rawData, batchStoreMessage); err != nil {
		return nil, err
	}

	for _, v := range batchStoreMessage.Message {
		if m, err := ProtoMessageToPublishMessage(v); err != nil {
			return nil, err
		} else {
			result = append(result, m)
		}
	}
	return result, nil
}

func (p *ProtoBufSerializer) Decode(rawData []byte) (*brokerpublish.Message, error) {
	var protoMessage = publishpb.StorePublishMessage{}

	if err := proto.Unmarshal(rawData, &protoMessage); err != nil {
		return nil, err
	}

	return ProtoMessageToPublishMessage(&protoMessage)
}

func PublishMessageToProtoMessage(m *brokerpublish.Message, buffer *bytes.Buffer) (*publishpb.StorePublishMessage, error) {
	var (
		cp  = controlPacketForStorage(m.GetControlPacket())
		err error
	)
	if cp != nil {
		_, err := wire.Write(buffer, cp, wire.EncodeOptions{})
		if err != nil {
			logger.Logger.Error().Err(err).Msg("write error")
			return nil, err
		}
	}

	createdTime := m.CreatedTime
	if createdTime == 0 {
		createdTime = time.Now().UnixNano()
	}
	expiredTime := m.ExpiredTime
	if expiredTime == 0 {
		if publish := m.GetPublish(); publish != nil && publish.Properties != nil && publish.Properties.MessageExpiry != nil {
			expiredTime = createdTime + int64(*publish.Properties.MessageExpiry)*int64(time.Second)
		}
	}

	return &publishpb.StorePublishMessage{
		MessageID:     m.MessageID.String(),
		PublishPacket: buffer.Bytes(),
		PubReceived:   false,
		CreatedTime:   createdTime,
		ExpiredTime:   expiredTime,
		SenderID:      m.SendClientID,
	}, err
}

func controlPacketForStorage(cp *packets.ControlPacket) *packets.ControlPacket {
	if cp == nil {
		return nil
	}
	publish, ok := cp.Content.(*packets.Publish)
	if !ok {
		return cp
	}
	clonedPublish := *publish
	clonedPublish.Payload = append([]byte(nil), publish.Payload...)
	if publish.Properties != nil {
		props := *publish.Properties
		props.CorrelationData = append([]byte(nil), publish.Properties.CorrelationData...)
		props.SubscriptionIdentifier = append([]int(nil), publish.Properties.SubscriptionIdentifier...)
		props.User = append([]packets.User(nil), publish.Properties.User...)
		clonedPublish.Properties = &props
	}
	if clonedPublish.QoS > 0 && clonedPublish.PacketID == 0 {
		// MQTT wire encoding requires a non-zero PacketID for QoS1/QoS2. The
		// value is only a storage encoding placeholder and is cleared on decode.
		clonedPublish.PacketID = 1
	}
	cloned := packets.NewControlPacket(packets.PUBLISH)
	cloned.Content = &clonedPublish
	return cloned
}

func ProtoMessageToPublishMessage(protoMessage *publishpb.StorePublishMessage) (*brokerpublish.Message, error) {
	// Support store-only notifications on the QoS>0 path where only MessageID is required.
	// In this case, PublishPacket can be empty and we must not attempt to parse MQTT packets (would return EOF).
	if len(protoMessage.PublishPacket) == 0 {
		uid, err := uuid.Parse(protoMessage.MessageID)
		if err != nil {
			return nil, err
		}
		return &brokerpublish.Message{
			SendClientID: protoMessage.SenderID,
			MessageID:    uid,
			PubReceived:  protoMessage.PubReceived,
			CreatedTime:  protoMessage.CreatedTime,
			ExpiredTime:  protoMessage.ExpiredTime,
		}, nil
	}

	var bf = bufferpool.ByteBufferPool.Get()
	defer bufferpool.ByteBufferPool.Put(bf)

	bf.Write(protoMessage.PublishPacket)
	ctl, err := wire.Decode(bf, wire.DecodeOptions{})
	if err != nil {
		return nil, err
	}

	if ctl == nil {
		return nil, fmt.Errorf("empty content")
	}

	if ctl.Content == nil {
		return nil, fmt.Errorf("empty content")
	}

	pub, ok := ctl.Content.(*packets.Publish)
	if !ok {
		return nil, fmt.Errorf("invalid content")
	}
	pub.PacketID = 0

	uid, err := uuid.Parse(protoMessage.MessageID)
	if err != nil {
		return nil, err
	}

	msg := &brokerpublish.Message{
		SendClientID: protoMessage.SenderID,
		MessageID:    uid,
		PubReceived:  protoMessage.PubReceived,
		CreatedTime:  protoMessage.CreatedTime,
		ExpiredTime:  protoMessage.ExpiredTime,
	}
	publishPacket := packets.NewControlPacket(packets.PUBLISH)
	publishPacket.Content = pub
	msg.SetControlPacket(publishPacket)

	return msg, nil

}
