package client

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	brokersession "github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

func (c *Client) restoreIncomingQoS2FromSession(ctx context.Context, sess *proto_session.Session) {
	_ = ctx
	if c == nil || c.QoS2 == nil || sess == nil {
		return
	}
	for _, unfinished := range sess.GetUnfinishedMessages() {
		if unfinished == nil || unfinished.GetIsOutgoing() {
			continue
		}
		if unfinished.GetState() != proto_session.UnfinishedMessage_WAITING_PUBREL {
			continue
		}
		if unfinished.GetQos() != 2 || unfinished.GetPacketID() == 0 || len(unfinished.GetPublishPacket()) == 0 {
			continue
		}

		cp, err := wire.Decode(bytes.NewBuffer(unfinished.GetPublishPacket()), wire.DecodeOptions{})
		if err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.metaString()).Uint32("packet_id", unfinished.GetPacketID()).Msg("restore incoming qos2: decode publish failed")
			continue
		}
		publish, ok := cp.Content.(*packets.Publish)
		if !ok || publish == nil {
			continue
		}
		if publish.PacketID == 0 {
			publish.PacketID = uint16(unfinished.GetPacketID())
		}
		if publish.QoS != 2 {
			continue
		}

		msg := &brokerpublish.Message{AckReasonCode: byte(unfinished.GetAckReasonCode())}
		msg.MessageID = uuid.New()
		msg.SetControlPacket(cp)
		msg.SetFromSession(true)
		c.QoS2.Store(msg)
	}
}

func (c *Client) persistIncomingQoS2WaitingPubrel(ctx context.Context, msg *brokerpublish.Message) error {
	if !c.shouldPersistIncomingQoS2() {
		return nil
	}
	unfinished := brokersession.ConvertToProtoUnfinishedMessage(msg, false)
	if unfinished == nil {
		return fmt.Errorf("incoming qos2 unfinished message is invalid")
	}
	packetID := unfinished.GetPacketID()
	if packetID == 0 {
		return fmt.Errorf("incoming qos2 packet id is required")
	}
	return c.component.sessionCenter.UpsertIncomingUnfinished(ctx, &proto_session.UpsertIncomingUnfinishedRequest{
		ClientID:          c.getID(),
		PacketID:          packetID,
		UnfinishedMessage: unfinished,
		OwnerToken:        c.getOwnerToken(),
		NowUnixNano:       time.Now().UnixNano(),
	})
}

func (c *Client) removeIncomingQoS2Unfinished(ctx context.Context, packetID uint16) error {
	if !c.shouldPersistIncomingQoS2() || packetID == 0 {
		return nil
	}
	return c.component.sessionCenter.RemoveIncomingUnfinished(ctx, &proto_session.RemoveIncomingUnfinishedRequest{
		ClientID:    c.getID(),
		PacketID:    uint32(packetID),
		OwnerToken:  c.getOwnerToken(),
		NowUnixNano: time.Now().UnixNano(),
	})
}

func (c *Client) shouldPersistIncomingQoS2() bool {
	return c != nil &&
		c.component != nil &&
		c.component.sessionCenter != nil &&
		c.getID() != "" &&
		c.sessionExpiryInterval > 0
}
