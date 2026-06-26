package client

import (
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetpool"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetsize"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

func (c *Client) Write(packet *clientcap.WritePacket) error {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()

	return c.write(packet)
}

func (c *Client) RetryWrite(packet *clientcap.WritePacket) error {
	if packet == nil {
		return fmt.Errorf("packet is nil")
	}

	// try lock after get token

	if _, ok := packet.Packet.Content.(*packets.Publish); ok {
		newPublishPacket := packetpool.PublishPool.Get()
		packetpool.CopyPublish(newPublishPacket, packet.Packet)

		// Set Dup flag to indicate this is a retransmission
		if publishContent, ok := newPublishPacket.Content.(*packets.Publish); ok {
			publishContent.Duplicate = true
		}

		// Replace the original packet with the new one
		packet.Packet = newPublishPacket

		// Put the packet back to pool after write completes
		defer packetpool.PublishPool.Put(newPublishPacket)
	}

	c.writeMux.Lock()
	defer c.writeMux.Unlock()

	return c.write(packet)
}

func (c *Client) write(packet *clientcap.WritePacket) error {
	if packet == nil {
		return fmt.Errorf("packet is nil")
	}

	//logger.Logger.Debug().Any("write packet", packet.Messages).Str("client", c.MetaString()).Msg("write packet")

	// publishAck, subscribeAck, unsubscribeAck should use the same packetID as the original packet

	c.prepareOutboundPacket(packet)
	c.applyProblemInfoPreference(packet.Packet)
	c.trimOutboundOptionalProperties(packet.Packet)
	if err := c.enforceOutboundMaxPacketSize(packet); err != nil {
		return err
	}

	clearDeadline := c.setWriteDeadline()
	defer clearDeadline()

	if _, err := wire.Write(c.conn, packet.Packet, wire.EncodeOptions{}); err != nil {
		if c.cancel != nil {
			c.cancel(err)
		}
		if logger.Logger != nil {
			logger.Logger.Info().Err(err).Str("client", c.MetaString()).Str("topic", outboundWriteTopic(packet.Packet)).Msg("write packet error")
		}
		return err
	}
	c.markAcceptedConnAck(packet.Packet)
	return nil
}

func (c *Client) setWriteDeadline() func() {
	if c == nil || c.conn == nil {
		return func() {}
	}
	timeout := c.writeTimeout()
	if timeout <= 0 {
		return func() {}
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(timeout))
	return func() {
		_ = c.conn.SetWriteDeadline(time.Time{})
	}
}

func (c *Client) prepareOutboundPacket(packet *clientcap.WritePacket) {
	switch p := packet.Packet.Content.(type) {

	case *packets.ConnAck:
		metric.RecordMQTTSentPacket(packets.CONNACK)
		c.doSendConnAckHook(p)

	case *packets.Suback:
		metric.RecordMQTTSentPacket(packets.SUBACK)
		c.doSendSubAckHook(p)

	case *packets.Unsuback:
		metric.RecordMQTTSentPacket(packets.UNSUBACK)
		c.doSendUnsubAckHook(p)

	case *packets.Publish:
		c.prepareOutboundPublish(packet, p)
		c.packetIdentifierIDTopic.SetPacketIDTopic(p.PacketID, packet.FullTopic)

	case *packets.Puback:
		metric.RecordMQTTSentPacket(packets.PUBACK)
		c.doSendPubAckHook(p)

	case *packets.Pubrec:
		metric.RecordMQTTSentPacket(packets.PUBREC)
		c.doSendPubRecHook(p)

	case *packets.Pubrel:
		metric.RecordMQTTSentPacket(packets.PUBREL)
		c.doSendPubRelHook(p)

	case *packets.Pubcomp:
		metric.RecordMQTTSentPacket(packets.PUBCOMP)
		c.doSendPubCompHook(p)

	case *packets.Pingresp:
		metric.RecordMQTTSentPacket(packets.PINGRESP)
		c.doSendPingRespHook(p)

	}
}

func (c *Client) prepareOutboundPublish(packet *clientcap.WritePacket, p *packets.Publish) {
	// MQTT5 Topic Alias (outbound).
	if c.topicAliasManager != nil {
		c.topicAliasManager.ApplyDownlink(p)
	}

	metric.RecordMQTTSentPacket(packets.PUBLISH)
	metric.RecordMQTTPublishPacket(p.QoS, "out")
	c.doSendPublishHook(p)
}

func (c *Client) doSendConnAckHook(p *packets.ConnAck) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendConnAck(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Msg("plugin DoSendConnAck error, continue sending")
	}
}

func (c *Client) doSendSubAckHook(p *packets.Suback) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendSubAck(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Msg("plugin DoSendSubAck error, continue sending")
	}
}

func (c *Client) doSendUnsubAckHook(p *packets.Unsuback) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendUnsubAck(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Msg("plugin DoSendUnsubAck error, continue sending")
	}
}

func (c *Client) doSendPublishHook(p *packets.Publish) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPublish(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Str("topic", p.Topic).Msg("plugin DoSendPublish error, continue sending")
	}
}

func (c *Client) doSendPubAckHook(p *packets.Puback) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPubAck(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Uint16("packetID", p.PacketID).Msg("plugin DoSendPubAck error, continue sending")
	}
}

func (c *Client) doSendPubRecHook(p *packets.Pubrec) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPubRec(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Uint16("packetID", p.PacketID).Msg("plugin DoSendPubRec error, continue sending")
	}
}

func (c *Client) doSendPubRelHook(p *packets.Pubrel) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPubRel(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Uint16("packetID", p.PacketID).Msg("plugin DoSendPubRel error, continue sending")
	}
}

func (c *Client) doSendPubCompHook(p *packets.Pubcomp) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPubComp(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Uint16("packetID", p.PacketID).Msg("plugin DoSendPubComp error, continue sending")
	}
}

func (c *Client) doSendPingRespHook(p *packets.Pingresp) {
	if c.component == nil || c.component.plugin == nil {
		return
	}
	if err := c.component.plugin.DoSendPingResp(c.ctx, c.ID, p); err != nil {
		logger.Logger.Error().Err(err).Str("client", c.MetaString()).Msg("plugin DoSendPingResp error, continue sending")
	}
}

func (c *Client) trimOutboundOptionalProperties(packet *packets.ControlPacket) {
	maxSize := c.outboundMaxPacketSize()
	if maxSize <= 0 || packet == nil || packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
		return
	}
	switch p := packet.Content.(type) {
	case *packets.ConnAck:
		c.trimConnAckPropertiesToFit(packet, p, maxSize)
	case *packets.Auth:
		c.trimAuthPropertiesToFit(packet, p, maxSize)
	case *packets.Disconnect:
		c.trimDisconnectPropertiesToFit(packet, p, maxSize)
	case *packets.Puback:
		c.trimPubackPropertiesToFit(packet, p, maxSize)
	case *packets.Pubrec:
		c.trimPubrecPropertiesToFit(packet, p, maxSize)
	case *packets.Pubrel:
		c.trimPubrelPropertiesToFit(packet, p, maxSize)
	case *packets.Pubcomp:
		c.trimPubcompPropertiesToFit(packet, p, maxSize)
	case *packets.Suback:
		c.trimSubackPropertiesToFit(packet, p, maxSize)
	case *packets.Unsuback:
		c.trimUnsubackPropertiesToFit(packet, p, maxSize)
	}
}

func (c *Client) trimConnAckPropertiesToFit(packet *packets.ControlPacket, p *packets.ConnAck, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	props := p.Properties
	if len(props.User) > 0 {
		props.User = nil
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ReasonString != "" {
		props.ReasonString = ""
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ResponseInfo != "" {
		props.ResponseInfo = ""
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ServerReference != "" && !connAckUsesServerReference(p.ReasonCode) {
		props.ServerReference = ""
	}
}

func (c *Client) trimAuthPropertiesToFit(packet *packets.ControlPacket, p *packets.Auth, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	props := p.Properties
	if len(props.User) > 0 {
		props.User = nil
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ReasonString != "" {
		props.ReasonString = ""
	}
}

func (c *Client) trimDisconnectPropertiesToFit(packet *packets.ControlPacket, p *packets.Disconnect, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	props := p.Properties
	if len(props.User) > 0 {
		props.User = nil
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ReasonString != "" {
		props.ReasonString = ""
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if props.ServerReference != "" && !disconnectUsesServerReference(p.ReasonCode) {
		props.ServerReference = ""
	}
}

func (c *Client) trimPubackPropertiesToFit(packet *packets.ControlPacket, p *packets.Puback, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimPubrecPropertiesToFit(packet *packets.ControlPacket, p *packets.Pubrec, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimPubrelPropertiesToFit(packet *packets.ControlPacket, p *packets.Pubrel, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimPubcompPropertiesToFit(packet *packets.ControlPacket, p *packets.Pubcomp, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimSubackPropertiesToFit(packet *packets.ControlPacket, p *packets.Suback, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimUnsubackPropertiesToFit(packet *packets.ControlPacket, p *packets.Unsuback, maxSize int) {
	if p == nil || p.Properties == nil {
		return
	}
	c.trimReasonAndUserPropertiesToFit(packet, maxSize, &p.Properties.ReasonString, &p.Properties.User)
}

func (c *Client) trimReasonAndUserPropertiesToFit(packet *packets.ControlPacket, maxSize int, reason *string, user *[]packets.User) {
	if reason == nil || user == nil {
		return
	}
	if len(*user) > 0 {
		*user = nil
		if packetsize.CheckPacketSizeWithMax(packet, maxSize) == nil {
			return
		}
	}
	if *reason != "" {
		*reason = ""
	}
}

func disconnectUsesServerReference(reasonCode byte) bool {
	return reasonCode == packets.DisconnectUseAnotherServer || reasonCode == packets.DisconnectServerMoved
}

func (c *Client) enforceOutboundMaxPacketSize(packet *clientcap.WritePacket) error {
	maxSize := c.outboundMaxPacketSize()
	if maxSize <= 0 {
		return nil
	}
	if err := packetsize.CheckPacketSizeWithMax(packet.Packet, maxSize); err != nil {
		if pubContent, ok := packet.Packet.Content.(*packets.Publish); ok {
			return c.discardOversizedOutboundPublish(err, packet, pubContent, maxSize)
		}
		return c.closeForOversizedOutboundPacket(err, packet, maxSize)
	}
	return nil
}

func (c *Client) outboundMaxPacketSize() int {
	if c.clientMaximumPacketSize != nil {
		return int(*c.clientMaximumPacketSize)
	}
	return 0
}

func (c *Client) discardOversizedOutboundPublish(
	err error,
	packet *clientcap.WritePacket,
	pubContent *packets.Publish,
	maxSize int,
) error {
	// MQTT5 §3.1.2.11.4：超出客户端 Maximum Packet Size 的 PUBLISH 必须被丢弃，
	// 不能因此断开连接，否则会丢失同连接上其它正在传输的报文。
	metric.DiscardOversizedOutboundPublish.WithLabelValues(publishQoSLabel(pubContent.QoS)).Inc()
	logger.Logger.Warn().
		Err(err).
		Str("client", c.MetaString()).
		Int("maxSize", maxSize).
		Str("topic", pubContent.Topic).
		Str("fullTopic", packet.FullTopic).
		Uint8("qos", pubContent.QoS).
		Msg("discard oversized outbound publish")
	c.revertDownlinkAlias(packet.FullTopic, pubContent.Topic)
	// QoS0：直接丢；QoS1/2：调用方负责把对应的 inflight token / cursor 推进。
	// 这里返回一个特定错误便于上层识别。
	return errOversizedOutboundPublish
}

func publishQoSLabel(qos byte) string {
	switch qos {
	case 1:
		return "1"
	case 2:
		return "2"
	default:
		return "0"
	}
}

func (c *Client) revertDownlinkAlias(fullTopic, publishTopic string) {
	if c.topicAliasManager == nil {
		return
	}
	originalTopic := fullTopic
	if originalTopic == "" {
		originalTopic = publishTopic
	}
	if originalTopic != "" {
		c.topicAliasManager.RevertDownlinkAlias(originalTopic)
	}
}

func (c *Client) closeForOversizedOutboundPacket(err error, packet *clientcap.WritePacket, maxSize int) error {
	logger.Logger.Warn().
		Err(err).
		Str("client", c.MetaString()).
		Int("maxSize", maxSize).
		Msg("packet size exceeds client maximum")
	if !c.canSendDisconnect() {
		return c.closeForProtocolViolation(err)
	}

	packetSize, calcErr := packetsize.GetPacketSize(packet.Packet)
	if calcErr != nil {
		packetSize = 0
	}
	disconnectPacket := disconnectForPacketTooLarge(packetSize, maxSize)
	c.applyProblemInfoPreference(disconnectPacket)
	if packetsize.CheckPacketSizeWithMax(disconnectPacket, maxSize) != nil {
		disconnectPacket = minimalDisconnectForPacketTooLarge()
	}
	if packetsize.CheckPacketSizeWithMax(disconnectPacket, maxSize) != nil {
		c.cancel(err)
		_ = c.close()
		return err
	}

	_, _ = wire.Write(c.conn, disconnectPacket, wire.EncodeOptions{})
	c.cancel(err)
	_ = c.close()
	return err
}

func outboundWriteTopic(packet *packets.ControlPacket) string {
	if packet == nil {
		return ""
	}
	publish, ok := packet.Content.(*packets.Publish)
	if !ok {
		return ""
	}
	return publish.Topic
}

func (c *Client) markAcceptedConnAck(packet *packets.ControlPacket) {
	if connAck, ok := packet.Content.(*packets.ConnAck); ok && !mqtt5ReasonCodeIsError(connAck.ReasonCode) {
		c.connAckAccepted.Store(true)
	}
}
