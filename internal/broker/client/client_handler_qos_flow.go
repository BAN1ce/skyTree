package client

import (
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// handlePubAck handles PUBACK for a downlink QoS1 PUBLISH.
//
// Both success and MQTT5 negative acknowledgements complete the matching outgoing inflight entry.
func (i *InnerHandler) handlePubAck(ctx context.Context, pubAck *packets.Puback) error {
	c := i.client
	if pubAck != nil && mqtt5ReasonCodeIsError(pubAck.ReasonCode) {
		logger.Logger.Warn().
			Str("client", c.metaString()).
			Uint16("packetID", pubAck.PacketID).
			Uint8("reason", pubAck.ReasonCode).
			Msg("received negative PUBACK; completing outgoing inflight as terminal failure")
		return i.completeTerminalOutgoingInflight(ctx, pubAck.PacketID, 1, "negative")
	}
	return i.completeTerminalOutgoingInflight(ctx, pubAck.PacketID, 1, "success")
}

// completeTerminalOutgoingInflight 完成一个已经到达终态的下行 inflight 消息。
//
// 它负责释放限流 token、推进可持久化游标，并唤醒 delivery runner 继续发送后续消息。
func (i *InnerHandler) completeTerminalOutgoingInflight(ctx context.Context, packetID uint16, qos byte, result string) error {
	c := i.client
	if c == nil || c.outgoingInflight == nil {
		return nil
	}
	ackedEntry, ok := c.outgoingInflight.MarkAcked(packetID, qos)
	if !ok {
		metric.RecordDuplicateDelivery("normal", "ack")
		return nil
	}
	observeDeliveryAckDelay(ackedEntry, result)
	// Release one downlink window token after the client acknowledges the packet.
	if c.publishBucket != nil && ackedEntry.FlowTokenHeld {
		c.publishBucket.PutToken()
	}
	i.advanceAckedOutgoingInflight(ctx)
	return nil
}

func observeDeliveryAckDelay(e *outgoingInflightEntry, result string) {
	if e == nil || e.Retained || e.FirstPubTime.IsZero() {
		return
	}
	metric.RecordDeliveryAckDelay(deliveryPathFromInflight(e), int(e.QoS), result, time.Since(e.FirstPubTime))
}

// advanceAckedOutgoingInflight advances acknowledged downlink message cursors in send order.
//
// Only contiguous acked inflight entries can advance the cursor, which avoids skipping unacked messages.
func (i *InnerHandler) advanceAckedOutgoingInflight(ctx context.Context) {
	c := i.client
	if c == nil || c.outgoingInflight == nil {
		return
	}
	entries := c.outgoingInflight.PopAckedInOrder()
	if len(entries) == 0 {
		return
	}
	// Only entries that were actually persisted to the session store (i.e. restored
	// from a previous session) need a RemoveOutgoingUnfinished raft write on ack.
	// Entries sent and acked within this online session were never persisted, so
	// removing them is a no-op; skipping avoids that raft write.
	// Persist each delivery cursor and clear matching unfinished/session/PacketID state.
	for _, e := range entries {
		c.packetIdentifierIDTopic.DeletePacketID(e.PacketID)
		if e.Retained {
			if e.PersistedInSession {
				go c.removeOutgoingUnfinishedFromSession(e.MessageID)
			}
			continue
		}

		cursorAdvanced := false
		if c.component != nil && c.component.deliveryCursorStore != nil {
			if err := c.component.deliveryCursorStore.AdvanceCursor(ctx, brokerstore.DeliveryCursor{
				UpdatedTS:  time.Now(),
				ClientID:   c.getID(),
				Generation: e.Generation,
				LastTS:     e.TaskTS,
				LastTaskID: e.TaskID,
			}); err != nil {
				logger.Logger.Warn().
					Err(err).
					Str("client", c.metaString()).
					Uint16("packetID", e.PacketID).
					Msg("append delivery cursor failed on terminal acknowledgement")
			} else {
				cursorAdvanced = true
			}
		}
		if cursorAdvanced {
			metric.RecordDeliveryCursorLag(deliveryPathFromInflight(e), time.Since(e.TaskTS))
			c.completeSharedDeliveryTask(ctx, e.ShareGroup, e.SharedTaskID)
		}
		if e.PersistedInSession {
			go c.removeOutgoingUnfinishedFromSession(e.MessageID)
		}
	}
	// Count these terminal downlink ACKs toward the periodic outgoing-progress commit so an
	// unexpected crash only forces a bounded replay (see commitOutgoingProgressToSession).
	c.recordOutgoingTerminalAcks(len(entries), c.deliveryRuntimeConfig())
	if c.deliveryWakeCh != nil {
		select {
		case c.deliveryWakeCh <- struct{}{}:
		default:
		}
	}
}

// handlePubRec handles PUBREC for a downlink QoS2 PUBLISH.
//
// Success advances the downlink inflight state and sends PUBREL; negative PUBREC terminates the inflight entry.
func (i *InnerHandler) handlePubRec(ctx context.Context, pubRec *packets.Pubrec) error {
	c := i.client
	if pubRec != nil && mqtt5ReasonCodeIsError(pubRec.ReasonCode) {
		logger.Logger.Warn().
			Str("client", c.metaString()).
			Uint16("packetID", pubRec.PacketID).
			Uint8("reason", pubRec.ReasonCode).
			Msg("received negative PUBREC; completing outgoing inflight as terminal failure")
		return i.completeTerminalOutgoingInflight(ctx, pubRec.PacketID, 2, "negative")
	}
	// Downlink QoS2 sends PUBREL after PUBREC, then waits for terminal PUBCOMP.
	if c != nil && c.outgoingInflight != nil {
		if e, ok := c.outgoingInflight.Get(pubRec.PacketID); ok && e != nil && e.QoS == 2 && e.State == outInflightWaitingPubRec {
			c.outgoingInflight.UpdateState(pubRec.PacketID, outInflightWaitingPubComp)
			pubRelCP := packets.NewControlPacket(packets.PUBREL)
			pubRelCP.Content = &packets.Pubrel{PacketID: pubRec.PacketID, ReasonCode: packets.PubrelSuccess}
			_ = c.write(&clientcap.WritePacket{Packet: pubRelCP})
		}
	}
	c.packetIdentifierIDTopic.DeletePacketID(pubRec.PacketID)
	return nil
}

// handlePubComp handles PUBCOMP for a downlink QoS2 PUBREL.
//
// PUBCOMP means the downlink QoS2 handshake is complete and the cursor can advance.
func (i *InnerHandler) handlePubComp(ctx context.Context, pubcomp *packets.Pubcomp) error {
	c := i.client
	if pubcomp != nil && mqtt5ReasonCodeIsError(pubcomp.ReasonCode) {
		logger.Logger.Warn().
			Str("client", c.metaString()).
			Uint16("packetID", pubcomp.PacketID).
			Uint8("reason", pubcomp.ReasonCode).
			Msg("received negative PUBCOMP; completing outgoing inflight as terminal failure")
		return i.completeTerminalOutgoingInflight(ctx, pubcomp.PacketID, 2, "negative")
	}
	return i.completeTerminalOutgoingInflight(ctx, pubcomp.PacketID, 2, "success")
}

// handlePubRel handles PUBREL in the client's inbound QoS2 flow.
//
// The stored PUBLISH enters the publish path only after PUBREL, then PUBCOMP is returned.
func (i *InnerHandler) handlePubRel(ctx context.Context, receivedPubRel *packets.Pubrel) error {
	var (
		publishComp = packets.NewControlPacket(packets.PUBCOMP)
		client      = i.client
	)
	publishComp.Content = &packets.Pubcomp{
		PacketID:   receivedPubRel.PacketID,
		ReasonCode: 0,
	}
	// Read the first-phase PUBLISH from QoS2 storage; remove state only after publish side effects commit.
	publishPacket, ok := i.client.QoS2.Read(receivedPubRel.PacketID)

	if !ok {
		logger.Logger.Warn().Str("client", client.getID()).Uint16("packetID", receivedPubRel.PacketID).Msg("qos2 handle pubrel error, packet id not found, maybe deleted, because handled")
		publishComp.Content = &packets.Pubcomp{
			PacketID:   receivedPubRel.PacketID,
			ReasonCode: packets.PubcompPacketIdentifierNotFound,
		}
		_ = client.write(&clientcap.WritePacket{Packet: publishComp})
		return nil
	}
	if mqtt5ReasonCodeIsError(receivedPubRel.ReasonCode) {
		logger.Logger.Warn().
			Str("client", client.getID()).
			Uint16("packetID", receivedPubRel.PacketID).
			Uint8("reason", receivedPubRel.ReasonCode).
			Msg("received negative PUBREL; completing incoming qos2 flow without delivery")
		if err := i.client.removeIncomingQoS2Unfinished(ctx, receivedPubRel.PacketID); err != nil {
			return err
		}
		i.client.QoS2.Delete(receivedPubRel.PacketID)
		_ = client.write(&clientcap.WritePacket{Packet: publishComp})
		return nil
	}

	publish := publishPacket.GetPublish()
	if publish == nil {
		logger.Logger.Warn().Str("client", client.getID()).Msg("qos2 handle pubrel error, packet publish is nil")
		if err := i.client.removeIncomingQoS2Unfinished(ctx, receivedPubRel.PacketID); err != nil {
			return err
		}
		i.client.QoS2.Delete(receivedPubRel.PacketID)
		_ = client.write(&clientcap.WritePacket{Packet: publishComp})
		return nil
	}
	storedPubrecReason := publishPacket.AckReasonCode
	if storedPubrecReason == 0 {
		storedPubrecReason = packets.PubrecSuccess
	}
	if mqtt5ReasonCodeIsError(storedPubrecReason) {
		logger.Logger.Warn().
			Str("client", client.getID()).
			Uint16("packetID", receivedPubRel.PacketID).
			Uint8("pubrecReason", storedPubrecReason).
			Msg("stored PUBREC reason is error; skipping qos2 publish side effects")
		if err := i.client.removeIncomingQoS2Unfinished(ctx, receivedPubRel.PacketID); err != nil {
			return err
		}
		i.client.QoS2.Delete(receivedPubRel.PacketID)
		_ = client.write(&clientcap.WritePacket{Packet: publishComp})
		return nil
	}

	// QoS2 retained messages commit at PUBREL. Commit retained state before live delivery
	// so client retries after retained failures do not duplicate downstream delivery.
	if publish.Retain {
		if err := i.commitRetainedPublish(publish, publish.Topic); err != nil {
			return err
		}
	}

	if storedPubrecReason == packets.PubrecNoMatchingSubscribers {
		logger.Logger.Debug().
			Str("client", client.getID()).
			Uint16("packetID", receivedPubRel.PacketID).
			Msg("skip qos2 live delivery because PUBREC reason is No Matching Subscribers")
		if err := i.client.removeIncomingQoS2Unfinished(ctx, receivedPubRel.PacketID); err != nil {
			return err
		}
		i.client.QoS2.Delete(receivedPubRel.PacketID)
		_ = client.write(&clientcap.WritePacket{Packet: publishComp})
		return nil
	}

	if i.client.component == nil || i.client.component.stateRouter == nil {
		return fmt.Errorf("state router is nil")
	}
	// QoS2 messages enter the delivery path at PUBREL to avoid first-phase duplicate delivery.
	livePublish := clonePublishForLiveDelivery(publish)
	if err := i.client.component.stateRouter.RoutePublish(client.getCtx(), staterouter.RoutePublishRequest{
		BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:     i.client.getID(),
		OwnerToken:   i.client.getOwnerToken(),
		Message: &brokerpublish.Message{
			Publish:      livePublish,
			SendClientID: i.client.getID(),
			Duplicate:    publish.Duplicate,
			MessageID:    publishPacket.MessageID,
		},
	}); err != nil {
		logger.Logger.Error().Err(err).Msg("messageStore publish packet error")
		return err
	}
	if err := i.client.removeIncomingQoS2Unfinished(ctx, receivedPubRel.PacketID); err != nil {
		return err
	}
	i.client.QoS2.Delete(receivedPubRel.PacketID)
	_ = client.write(&clientcap.WritePacket{Packet: publishComp})
	return nil

}

// handlePing 响应客户端 PINGREQ，复用全局 PINGRESP 包减少分配。
func (i *InnerHandler) handlePing(ctx context.Context, _ *packets.Pingreq) error {
	return i.client.write(&clientcap.WritePacket{
		Packet: pingResp,
	})
}
