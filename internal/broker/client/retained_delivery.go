package client

import (
	"bytes"
	"context"
	"time"

	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/google/uuid"
)

func (c *Client) writeRetainedPublish(ctx context.Context, cp *packets.ControlPacket, fullTopic string) {
	if c == nil || cp == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	publish, ok := cp.Content.(*packets.Publish)
	if !ok || publish == nil {
		return
	}
	// MQTT5 §3.3.2.3.2：投递前再次校验 PFI=1 时 payload 是否仍是合法 UTF-8。
	// retained 报文会经过持久化与重建，这里做最终兜底校验。
	if outboundPayloadFormatInvalid(publish) {
		return
	}
	if publish.QoS == 0 {
		publish.PacketID = 0
		_ = c.Write(&clientcap.WritePacket{Packet: cp, FullTopic: fullTopic})
		return
	}

	publish.PacketID = c.NextPacketID()
	if publish.PacketID == 0 {
		return
	}
	if c.publishBucket != nil {
		c.publishBucket.GetToken(ctx)
	}

	now := time.Now()
	messageID := uuid.New()
	state := outInflightWaitingPubAck
	if publish.QoS == 2 {
		state = outInflightWaitingPubRec
	}
	var buf bytes.Buffer
	_, _ = wire.Write(&buf, cp, wire.EncodeOptions{})
	if c.outgoingInflight != nil {
		replaced := c.outgoingInflight.Put(&outgoingInflightEntry{
			PacketID:      publish.PacketID,
			QoS:           publish.QoS,
			TaskTS:        now,
			TaskID:        messageID,
			MessageID:     messageID,
			Retained:      true,
			PublishPacket: append([]byte(nil), buf.Bytes()...),
			FirstPubTime:  now,
			LastSendTime:  now,
			FlowTokenHeld: c.publishBucket != nil,
			State:         state,
		})
		if replaced {
			if c.publishBucket != nil {
				c.publishBucket.PutToken()
			}
			_ = c.write(&clientcap.WritePacket{Packet: disconnectForProtocolError("downlink packet id reused while retained inflight")})
			_ = c.close()
			return
		}
	}

	if err := c.Write(&clientcap.WritePacket{Packet: cp, FullTopic: fullTopic}); err != nil {
		if c.outgoingInflight != nil {
			if _, ok := c.outgoingInflight.Delete(publish.PacketID); ok && c.publishBucket != nil {
				c.publishBucket.PutToken()
			}
		} else if c.publishBucket != nil {
			c.publishBucket.PutToken()
		}
	}
}
