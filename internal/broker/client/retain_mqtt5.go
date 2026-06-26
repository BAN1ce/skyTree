package client

import (
	"bytes"
	"fmt"
	"time"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_retain"
)

func NewRetainMessageFromPublish(publish *packets.Publish, now time.Time, publisherClientID string) (*proto_retain.RetainMessage, error) {
	return newRetainMessageFromPublish(publish, now, publisherClientID)
}

func newRetainMessageFromPublish(publish *packets.Publish, now time.Time, publisherClientID string) (*proto_retain.RetainMessage, error) {
	if publish == nil {
		return nil, fmt.Errorf("publish is nil")
	}

	cp := packets.NewControlPacket(packets.PUBLISH)
	cp.Content = retainPublishForWireEncoding(publish)

	var buf bytes.Buffer
	if _, err := wire.Write(&buf, cp, wire.EncodeOptions{}); err != nil {
		return nil, err
	}

	payload := make([]byte, len(publish.Payload))
	copy(payload, publish.Payload)

	expiredAtUnixNano := retainedExpiredAtUnixNano(publish, now)
	return &proto_retain.RetainMessage{
		Topic:             publish.Topic,
		Payload:           payload,
		Qos:               int32(publish.QoS),
		PublishPacket:     append([]byte(nil), buf.Bytes()...),
		CreatedAtUnixNano: now.UnixNano(),
		PublisherClientID: publisherClientID,
		ExpiredAtUnixNano: expiredAtUnixNano,
	}, nil
}

func retainPublishForWireEncoding(publish *packets.Publish) *packets.Publish {
	cloned := clonePublish(publish)
	if cloned != nil && cloned.QoS > 0 && cloned.PacketID == 0 {
		// Retained storage uses MQTT wire bytes, which require a PacketID for
		// QoS1/QoS2. The decoded retained publish clears this placeholder.
		cloned.PacketID = 1
	}
	return cloned
}

func publishFromRetainMessage(message *proto_retain.RetainMessage, now time.Time) (*packets.Publish, bool) {
	if message == nil || message.GetTopic() == "" {
		return nil, false
	}

	var publish *packets.Publish
	if len(message.GetPublishPacket()) > 0 {
		cp, err := wire.Decode(bytes.NewBuffer(message.GetPublishPacket()), wire.DecodeOptions{})
		if err != nil {
			return nil, false
		}
		var ok bool
		publish, ok = cp.Content.(*packets.Publish)
		if !ok {
			return nil, false
		}
	} else {
		publish = &packets.Publish{
			Topic:   message.GetTopic(),
			Payload: append([]byte(nil), message.GetPayload()...),
			QoS:     byte(message.GetQos()),
		}
	}

	publish.Topic = message.GetTopic()
	publish.Retain = true
	publish.PacketID = 0
	if publish.Properties == nil {
		publish.Properties = &packets.PublishProperties{}
	}
	if !applyRetainedMessageExpiry(publish, message.GetExpiredAtUnixNano(), now) {
		return nil, false
	}
	return publish, true
}

func retainedExpiredAtUnixNano(publish *packets.Publish, now time.Time) int64 {
	if publish == nil || publish.Properties == nil || publish.Properties.MessageExpiry == nil {
		return 0
	}
	return now.Add(time.Duration(*publish.Properties.MessageExpiry) * time.Second).UnixNano()
}

func applyRetainedMessageExpiry(publish *packets.Publish, expiredAtUnixNano int64, now time.Time) bool {
	if publish == nil || publish.Properties == nil || publish.Properties.MessageExpiry == nil {
		return true
	}
	if expiredAtUnixNano <= 0 {
		return false
	}
	deadline := time.Unix(0, expiredAtUnixNano)
	if !now.Before(deadline) {
		return false
	}
	remaining := uint32(deadline.Sub(now) / time.Second)
	if remaining == 0 {
		remaining = 1
	}
	publish.Properties.MessageExpiry = &remaining
	return true
}
