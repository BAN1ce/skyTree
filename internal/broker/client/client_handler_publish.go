package client

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	"github.com/google/uuid"
)

// handlePublish 处理客户端上行 PUBLISH。
//
// 该方法覆盖 Topic Alias、Topic Name 校验、ACL、服务端能力限制、QoS 去重、
// Receive Maximum、retain 存储、消息入库/投递和 QoS 应答。
func (i *InnerHandler) handlePublish(ctx context.Context, p *packets.Publish) error {
	if p == nil {
		return nil
	}
	topic, handled, err := i.preparePublishPacket(p)
	if handled || err != nil {
		return err
	}
	if handled, err := i.handleRepeatedPublish(p); handled || err != nil {
		return err
	}
	if i.rejectPublishByPlugin(ctx, p) {
		return nil
	}
	if err := i.enforcePublishCapabilities(p); err != nil {
		return err
	}
	if err := i.enforceIncomingPublishRate(); err != nil {
		return err
	}
	cleanup, err := i.trackIncomingPublishQuota(p)
	if err != nil {
		return err
	}
	defer cleanup()

	response, err := i.buildPublishResponse(ctx, p, topic)
	if err != nil {
		return err
	}
	if err := i.processRetainedPublish(p, topic); err != nil {
		return err
	}
	if err := i.dispatchLivePublish(p, topic); err != nil {
		return err
	}
	return i.writePublishResponse(response)
}

// preparePublishPacket 预处理上行 PUBLISH（规范化、Topic Alias 与基础校验）。
func (i *InnerHandler) preparePublishPacket(p *packets.Publish) (string, bool, error) {
	normalizeInboundPublishFlags(p)
	if p.Duplicate {
		logger.Logger.Info().Str("topic", p.Topic).Msg("duplicated message")
	}
	topic, err := i.applyPublishTopicAlias(p)
	if err != nil {
		return "", false, err
	}
	if err := i.validatePublishEnvelope(p, topic); err != nil {
		return "", false, err
	}
	if publishPayloadFormatInvalid(p) {
		return "", true, i.rejectPublishPayloadFormatInvalid(p)
	}
	return topic, false, nil
}

// normalizeInboundPublishFlags 清理内部路径可能带入的非法 DUP/PacketID 组合。
func normalizeInboundPublishFlags(p *packets.Publish) {
	if p == nil {
		return
	}
	// 网络入口已经由 wire 层拒绝 QoS0+DUP；这里额外保护直接调用 handlePublish
	// 的内部路径（测试、回放、插件等），避免非法标志污染 retain/live 路径。
	if p.QoS == subscription.QoS0 {
		p.Duplicate = false
		p.PacketID = 0
	}
}

// applyPublishTopicAlias 解析并应用客户端上行 Topic Alias。
func (i *InnerHandler) applyPublishTopicAlias(p *packets.Publish) (string, error) {
	finalTopic, updated := p.Topic, false
	var aliasErr error
	if i.client.topicAliasManager != nil {
		brokerCfg := i.client.brokerRuntimeConfig()
		finalTopic, updated, aliasErr = i.client.topicAliasManager.ApplyUplink(
			p,
			uint16(brokerCfg.ConnectAckProperty.TopicAliasMaximum),
		)
	}
	if aliasErr != nil {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForTopicAliasError(aliasErr)})
		_ = i.client.close()
		return "", aliasErr
	}
	if updated {
		p.Topic = finalTopic
	}
	return finalTopic, nil
}

// validatePublishEnvelope 校验 topic、属性和协议层约束。
func (i *InnerHandler) validatePublishEnvelope(p *packets.Publish, topic string) error {
	if topic == "" {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForTopicNameInvalid("empty topic name without a valid topic alias")})
		_ = i.client.close()
		return ErrProtocolError
	}
	if topicutil.HasWildcard(topic) {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForTopicNameInvalid("topic name contains wildcard")})
		_ = i.client.close()
		return ErrProtocolError
	}
	if p.Properties != nil && len(p.Properties.SubscriptionIdentifier) > 0 {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForProtocolError("Subscription Identifier is not allowed in client PUBLISH")})
		_ = i.client.close()
		return ErrProtocolError
	}
	return nil
}

// rejectPublishByPlugin 执行发布插件钩子并处理拒绝场景。
func (i *InnerHandler) rejectPublishByPlugin(ctx context.Context, p *packets.Publish) bool {
	if i.client.component == nil || i.client.component.plugin == nil {
		return false
	}
	if err := i.client.component.plugin.DoReceivedPublish(ctx, i.client.getID(), p); err != nil {
		i.denyPublish(p)
		return true
	}
	return false
}

// enforcePublishCapabilities 校验客户端请求是否超出服务端能力边界。
func (i *InnerHandler) enforcePublishCapabilities(p *packets.Publish) error {
	brokerCfg := i.client.brokerRuntimeConfig()
	prop := &brokerCfg.ConnectAckProperty
	if int(p.QoS) > prop.MaxQos {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForQoSNotSupported("publish QoS exceeds server Maximum QoS")})
		_ = i.client.close()
		return ErrProtocolError
	}
	if prop.RetainAvailable == 0 && p.Retain {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForRetainNotSupported("retained messages are disabled")})
		_ = i.client.close()
		return ErrProtocolError
	}
	return nil
}

// enforceIncomingPublishRate 检查上行消息速率是否超过限制。
func (i *InnerHandler) enforceIncomingPublishRate() error {
	if i == nil || i.client == nil || i.client.incomingPublishRateLimiter == nil {
		return nil
	}
	if i.client.incomingPublishRateLimiter.allow(time.Now()) {
		return nil
	}
	_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForMessageRateTooHigh("incoming publish rate exceeded")})
	_ = i.client.close()
	return ErrProtocolError
}

// handleRepeatedPublish 处理 QoS2 重复上行 PUBLISH 的协议响应。
func (i *InnerHandler) handleRepeatedPublish(p *packets.Publish) (bool, error) {
	if p == nil || p.QoS != subscription.QoS2 || i.client == nil || i.client.QoS2 == nil || p.PacketID == 0 {
		return false, nil
	}
	stored, ok := i.client.QoS2.Read(p.PacketID)
	if !ok {
		return false, nil
	}
	reasonCode := byte(packets.PubrecSuccess)
	if stored != nil && stored.AckReasonCode != 0 {
		reasonCode = stored.AckReasonCode
	}
	// MQTT-4.3.3-9: if the receiver has sent a PUBREC with reason >= 0x80,
	// any subsequent PUBLISH with the same PacketID must be treated as a new message.
	if mqtt5ReasonCodeIsError(reasonCode) {
		return false, nil
	}
	pubRec := packets.NewControlPacket(packets.PUBREC)
	pubRec.Content = &packets.Pubrec{PacketID: p.PacketID, ReasonCode: reasonCode}
	_ = i.client.write(&clientcap.WritePacket{Packet: pubRec})
	return true, nil
}

// trackIncomingPublishQuota 跟踪上行 inflight 配额并返回清理函数。
func (i *InnerHandler) trackIncomingPublishQuota(p *packets.Publish) (func(), error) {
	if p.QoS != subscription.QoS1 && p.QoS != subscription.QoS2 {
		return func() {}, nil
	}
	inflight := len(i.client.incomingQoS1Inflight)
	if i.client.QoS2 != nil {
		inflight += i.client.QoS2.Count()
	}
	maximum := int(i.client.serverReceiveMaximum)
	if maximum > 0 && inflight >= maximum {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForReceiveMaximumExceeded(inflight, maximum)})
		_ = i.client.close()
		return func() {}, ErrProtocolError
	}
	if p.QoS != subscription.QoS1 || p.PacketID == 0 {
		return func() {}, nil
	}
	i.client.incomingQoS1Inflight[p.PacketID] = struct{}{}
	return func() {
		delete(i.client.incomingQoS1Inflight, p.PacketID)
	}, nil
}

// processRetainedPublish 在满足条件时写入 retained 存储。
func (i *InnerHandler) processRetainedPublish(p *packets.Publish, topic string) error {
	if !p.Retain || p.QoS == subscription.QoS2 {
		return nil
	}
	return i.commitRetainedPublish(p, topic)
}

// commitRetainedPublish 负责 retained 消息的写入、覆盖和删除逻辑。
func (i *InnerHandler) commitRetainedPublish(p *packets.Publish, topic string) error {
	if i.client == nil || i.client.component == nil || i.client.component.retain == nil {
		return errors.New("retain store is nil")
	}
	if len(p.Payload) == 0 {
		if err := i.client.component.retain.DeleteRetainMessage(topic); err != nil {
			logger.Logger.Error().Err(err).Str("id", i.client.getID()).Str("topic", topic).Msg("delete retain message error")
			return err
		}
		return nil
	}
	retained, err := newRetainMessageFromPublish(p, time.Now(), i.client.getID())
	if err != nil {
		logger.Logger.Error().Err(err).Str("id", i.client.getID()).Str("topic", topic).Msg("retain message encode error")
		return err
	}
	if err := i.client.component.retain.PutRetainMessage(retained); err != nil {
		logger.Logger.Error().Err(err).Str("id", i.client.getID()).Str("topic", topic).Msg("retain message error")
		return err
	}
	return nil
}

// buildPublishResponse 构造 QoS1/QoS2 场景下返回给客户端的 ACK 报文。
func (i *InnerHandler) buildPublishResponse(ctx context.Context, p *packets.Publish, topic string) (*packets.ControlPacket, error) {
	noMatchingSubscribers := i.noMatchingSubscribers(ctx, topic)
	switch p.QoS {
	case subscription.QoS1:
		reasonCode := byte(packets.PubackSuccess)
		if noMatchingSubscribers {
			reasonCode = packets.PubackNoMatchingSubscribers
		}
		pubAck := packets.NewControlPacket(packets.PUBACK)
		pubAck.Content = &packets.Puback{PacketID: p.PacketID, ReasonCode: reasonCode}
		return pubAck, nil
	case subscription.QoS2:
		reasonCode := byte(packets.PubrecSuccess)
		if noMatchingSubscribers {
			reasonCode = packets.PubrecNoMatchingSubscribers
		}
		if _, err := i.storeQoS2Publish(ctx, p, reasonCode); err != nil {
			return nil, err
		}
		pubRec := packets.NewControlPacket(packets.PUBREC)
		pubRec.Content = &packets.Pubrec{PacketID: p.PacketID, ReasonCode: reasonCode}
		return pubRec, nil
	default:
		return nil, nil
	}
}

// storeQoS2Publish 暂存 QoS2 首阶段 PUBLISH，等待 PUBREL 再提交副作用。
func (i *InnerHandler) storeQoS2Publish(ctx context.Context, p *packets.Publish, reasonCode byte) (*brokerpublish.Message, error) {
	publishCP := packets.NewControlPacket(packets.PUBLISH)
	publishCP.Content = p
	msg := &brokerpublish.Message{
		AckReasonCode: reasonCode,
		MessageID:     i.newIncomingDeliveryMessageID(),
	}
	msg.SetControlPacket(publishCP)
	i.client.QoS2.Store(msg)
	if err := i.client.persistIncomingQoS2WaitingPubrel(ctx, msg); err != nil {
		i.client.QoS2.Delete(p.PacketID)
		return nil, err
	}
	return msg, nil
}

// dispatchLivePublish 将消息投递到在线下游分发链路。
func (i *InnerHandler) dispatchLivePublish(p *packets.Publish, topic string) error {
	if p.QoS == subscription.QoS2 {
		return nil
	}
	if i.client.component == nil || i.client.component.stateRouter == nil {
		return fmt.Errorf("state router is nil")
	}
	livePublish := clonePublishForLiveDelivery(p)
	if err := i.client.component.stateRouter.RoutePublish(i.client.getCtx(), staterouter.RoutePublishRequest{
		BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:     i.client.getID(),
		OwnerToken:   i.client.getOwnerToken(),
		Message: &brokerpublish.Message{
			Publish:      livePublish,
			SendClientID: i.client.getID(),
			Duplicate:    p.Duplicate,
			MessageID:    i.qos1DeliveryMessageID(p),
		},
	}); err != nil {
		logger.Logger.Error().Str("topic", topic).Msg(err.Error())
		return ErrStore
	}
	return nil
}

// writePublishResponse 发送 PUBACK/PUBREC 并清理 QoS1 临时关联状态。
func (i *InnerHandler) writePublishResponse(response *packets.ControlPacket) error {
	if response == nil {
		return nil
	}
	if err := i.client.write(&clientcap.WritePacket{Packet: response}); err != nil {
		logger.Logger.Warn().Err(err).Msg("write publish response error")
		return err
	}
	i.clearQoS1DeliveryMessageID(response)
	return nil
}

// qos1DeliveryMessageID 获取或创建 QoS1 消息对应的内部 message id。
func (i *InnerHandler) qos1DeliveryMessageID(p *packets.Publish) uuid.UUID {
	if i == nil || i.client == nil || p == nil || p.QoS != subscription.QoS1 || p.PacketID == 0 {
		return uuid.Nil
	}
	if id, ok := i.client.incomingQoS1MessageIDs[p.PacketID]; ok {
		return id
	}
	id := i.newIncomingDeliveryMessageID()
	i.client.incomingQoS1MessageIDs[p.PacketID] = id
	return id
}

// clearQoS1DeliveryMessageID 在 PUBACK 完成后删除 QoS1 message id 映射。
func (i *InnerHandler) clearQoS1DeliveryMessageID(response *packets.ControlPacket) {
	if i == nil || i.client == nil || response == nil || response.Type != packets.PUBACK {
		return
	}
	pubAck, ok := response.Content.(*packets.Puback)
	if !ok || pubAck.PacketID == 0 {
		return
	}
	delete(i.client.incomingQoS1MessageIDs, pubAck.PacketID)
}

// newIncomingDeliveryMessageID 生成入站投递链路使用的消息唯一标识。
func (i *InnerHandler) newIncomingDeliveryMessageID() uuid.UUID {
	return uuid.New()
}

// noMatchingSubscribers 判断当前 topic 是否没有任何匹配订阅者。
//
// 只有配置开启 NoSubTopicResponse 时才查询 sub-center，否则保持旧行为不返回 No Matching Subscribers。
func (i *InnerHandler) noMatchingSubscribers(ctx context.Context, topic string) bool {
	brokerCfg := i.client.brokerRuntimeConfig()
	if brokerCfg.NoSubTopicResponse == 0 {
		return false
	}
	if i == nil || i.client == nil || i.client.component == nil || i.client.component.stateRouter == nil {
		logger.Logger.Warn().Str("topic", topic).Msg("state router is nil while checking matching subscribers")
		return false
	}
	matched, err := i.client.component.stateRouter.HasMatchingSubscribers(ctx, staterouter.HasMatchingSubscribersRequest{
		BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:     i.client.getID(),
		Topic:        topic,
	})
	if err != nil {
		logger.Logger.Warn().Err(err).Str("topic", topic).Msg("failed to check matching subscribers")
		return false
	}
	return !matched
}

// denyPublish 在 ACL/插件拒绝 PUBLISH 后按 QoS 生成协议层拒绝响应。
func (i *InnerHandler) denyPublish(p *packets.Publish) {
	if p == nil {
		return
	}
	switch p.QoS {
	case subscription.QoS0:
		// MQTT5 §3.3.4：QoS0 PUBLISH 没有反向 ACK 报文。被 ACL 拒绝时
		// 默认按 spec 推荐的"丢弃"策略，避免误关连接。可通过配置 acl.qos0_reject_policy=disconnect 改为断开。
		policy := "drop"
		if brokerCfg := i.client.brokerRuntimeConfig(); brokerCfg.ACL.QoS0RejectPolicy != "" {
			policy = brokerCfg.ACL.QoS0RejectPolicy
		}
		if policy == "disconnect" {
			_ = i.client.write(&clientcap.WritePacket{Packet: newServerDisconnect(packets.DisconnectNotAuthorized, "publish not authorized")})
			_ = i.client.close()
		}
		// drop: 静默丢弃，不影响其它进行中的报文。
	case subscription.QoS1:
		cp := packets.NewControlPacket(packets.PUBACK)
		cp.Content = &packets.Puback{
			PacketID:   p.PacketID,
			ReasonCode: packets.PubackNotAuthorized,
			Properties: &packets.PubackProperties{ReasonString: "publish not authorized"},
		}
		_ = i.client.write(&clientcap.WritePacket{Packet: cp})
	case subscription.QoS2:
		cp := packets.NewControlPacket(packets.PUBREC)
		cp.Content = &packets.Pubrec{
			PacketID:   p.PacketID,
			ReasonCode: packets.PubrecNotAuthorized,
			Properties: &packets.PubrecProperties{ReasonString: "publish not authorized"},
		}
		_ = i.client.write(&clientcap.WritePacket{Packet: cp})
	}
}

// publishPayloadFormatInvalid 校验 Payload Format Indicator=1 时载荷是否为 UTF-8。
func publishPayloadFormatInvalid(p *packets.Publish) bool {
	if p == nil || p.Properties == nil || p.Properties.PayloadFormat == nil {
		return false
	}
	return *p.Properties.PayloadFormat == 1 && !utf8.Valid(p.Payload)
}

// rejectPublishPayloadFormatInvalid 根据 QoS 返回 Payload Format Invalid 对应响应。
func (i *InnerHandler) rejectPublishPayloadFormatInvalid(p *packets.Publish) error {
	switch p.QoS {
	case subscription.QoS0:
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForPayloadFormatInvalid("payload is not valid UTF-8")})
		_ = i.client.close()
		return ErrProtocolError
	case subscription.QoS1:
		cp := packets.NewControlPacket(packets.PUBACK)
		cp.Content = &packets.Puback{PacketID: p.PacketID, ReasonCode: packets.PubackPayloadFormatInvalid}
		return i.client.write(&clientcap.WritePacket{Packet: cp})
	case subscription.QoS2:
		cp := packets.NewControlPacket(packets.PUBREC)
		cp.Content = &packets.Pubrec{PacketID: p.PacketID, ReasonCode: packets.PubrecPayloadFormatInvalid}
		return i.client.write(&clientcap.WritePacket{Packet: cp})
	default:
		return nil
	}
}

// handleInitialEnhancedAuth 处理 CONNECT 中携带的 MQTT5 增强认证首轮数据。
//
// 返回 waitingForAuth=true 表示需要先发 AUTH 给客户端并暂停连接完成流程。
func (i *InnerHandler) handleInitialEnhancedAuth(
	ctx context.Context,
	connectPacket *packets.Connect,
	conAck *packets.ControlPacket,
	conAckContent *packets.ConnAck,
	assignedByServer bool,
) (bool, error) {
	if connectPacket == nil || connectPacket.Properties == nil || connectPacket.Properties.AuthMethod == "" {
		return false, nil
	}
	if i.client.enhancedAuthState == enhancedAuthAuthenticated {
		return false, nil
	}

	// 将 CONNECT 转成插件可识别的 AUTH 请求，由插件决定成功、继续认证或拒绝。
	i.client.enhancedAuthMethod = connectPacket.Properties.AuthMethod
	authReq := authPacketFromConnect(connectPacket)
	authResp, err := i.receiveAuthResponse(ctx, i.client.getID(), authReq)
	if err != nil {
		conAckContent.ReasonCode = packets.ConnAckNotAuthorized
		if errors.Is(err, ErrAuthHandlerNotSet) {
			conAckContent.ReasonCode = packets.ConnAckBadAuthenticationMethod
		}
		conAckContent.Properties = ensureConnAckProperties(conAckContent.Properties)
		conAckContent.Properties.ReasonString = "enhanced authentication failed: " + err.Error()
		_ = i.client.write(&clientcap.WritePacket{Packet: conAck})
		_ = i.client.close()
		return false, err
	}
	normalizeAuthResponse(authResp, i.client.enhancedAuthMethod)

	// 根据插件返回的 ReasonCode 推进连接状态机。
	switch authResp.ReasonCode {
	case packets.AuthSuccess:
		i.client.enhancedAuthState = enhancedAuthAuthenticated
		i.client.enhancedAuthData = cloneBytes(authResp.Properties.AuthData)
		return false, nil
	case packets.AuthContinueAuthentication:
		i.client.enhancedAuthState = enhancedAuthAuthenticating
		i.client.pendingEnhancedAuthConnect = connectPacket
		i.client.pendingEnhancedAuthAssignedByServer = assignedByServer
		return true, i.sendAuthPacket(ctx, authResp)
	default:
		conAckContent.ReasonCode = packets.ConnAckNotAuthorized
		conAckContent.Properties = ensureConnAckProperties(conAckContent.Properties)
		conAckContent.Properties.ReasonString = fmt.Sprintf("enhanced authentication rejected with reason code 0x%X", authResp.ReasonCode)
		if authResp.Properties != nil && authResp.Properties.ReasonString != "" {
			conAckContent.Properties.ReasonString = authResp.Properties.ReasonString
		}
		_ = i.client.write(&clientcap.WritePacket{Packet: conAck})
		_ = i.client.close()
		return false, ErrProtocolError
	}
}

// ensureConnAckProperties 在 properties 为 nil 时返回新分配的 ConnAckProperties。
func ensureConnAckProperties(p *packets.ConnAckProperties) *packets.ConnAckProperties {
	if p == nil {
		return &packets.ConnAckProperties{}
	}
	return p
}

// authPacketFromConnect 把 CONNECT 的 Auth Method/Data 转换成内部 AUTH 请求。
func authPacketFromConnect(connectPacket *packets.Connect) *packets.Auth {
	authPacket := &packets.Auth{
		ReasonCode: packets.AuthSuccess,
		Properties: &packets.AuthProperties{
			AuthMethod: connectPacket.Properties.AuthMethod,
		},
	}
	if len(connectPacket.Properties.AuthData) > 0 {
		authPacket.Properties.AuthData = make([]byte, len(connectPacket.Properties.AuthData))
		copy(authPacket.Properties.AuthData, connectPacket.Properties.AuthData)
	}
	return authPacket
}

// normalizeAuthResponse 补齐插件返回 AUTH 包中缺失的 Properties/AuthMethod。
func normalizeAuthResponse(authResp *packets.Auth, method string) {
	if authResp == nil {
		return
	}
	if authResp.Properties == nil {
		authResp.Properties = &packets.AuthProperties{}
	}
	if authResp.Properties.AuthMethod == "" {
		authResp.Properties.AuthMethod = method
	}
}

// receiveAuthResponse 调用增强认证插件并统一记录认证指标。
func (i *InnerHandler) receiveAuthResponse(ctx context.Context, clientID string, authPacket *packets.Auth) (*packets.Auth, error) {
	if !i.hasEnhancedAuthHandler() {
		metric.RecordAuthRequest("none", "failed")
		return nil, ErrAuthHandlerNotSet
	}
	authResp, err := i.client.component.plugin.DoReceivedAuth(ctx, clientID, authPacket)
	if err != nil {
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("plugin AUTH handling failed")
		metric.RecordAuthRequest("plugin", "failed")
		return nil, err
	}
	if authResp == nil {
		logger.Logger.Warn().Str("client", clientID).Msg("plugin returned nil AUTH response")
		metric.RecordAuthRequest("plugin", "failed")
		return nil, ErrAuthHandlerNotSet
	}
	if authResp.ReasonCode == packets.AuthSuccess {
		metric.RecordAuthRequest("plugin", "success")
	} else {
		metric.RecordAuthRequest("plugin", "failed")
	}
	return authResp, nil
}

// hasEnhancedAuthHandler 判断当前组件是否注册了增强认证处理器。
func (i *InnerHandler) hasEnhancedAuthHandler() bool {
	return i != nil &&
		i.client != nil &&
		i.client.component != nil &&
		i.client.component.plugin != nil &&
		len(i.client.component.plugin.OnReceivedAuth) > 0
}

// sendAuthPacket 发送 AUTH 响应，并在发送前执行 OnSendAuth 插件钩子。
func (i *InnerHandler) sendAuthPacket(ctx context.Context, authResp *packets.Auth) error {
	if i.client.component != nil && i.client.component.plugin != nil {
		if err := i.client.component.plugin.DoSendAuth(ctx, i.client.getID(), authResp); err != nil {
			logger.Logger.Warn().Err(err).Str("client", i.client.getID()).Msg("plugin OnSendAuth error")
		}
	}

	authRespPacket := packets.NewControlPacket(packets.AUTH)
	authRespPacket.Content = authResp
	metric.RecordMQTTSentPacket(packets.AUTH)
	return i.client.write(&clientcap.WritePacket{Packet: authRespPacket})
}

// disconnectForPayloadFormatInvalid 构造 Payload Format Invalid 的 DISCONNECT 包。
func disconnectForPayloadFormatInvalid(reason string) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	disc := &packets.Disconnect{
		ReasonCode: packets.DisconnectPayloadFormatInvalid,
	}
	if reason != "" {
		disc.Properties = &packets.DisconnectProperties{
			ReasonString: fmt.Sprintf("payload format invalid: %s", reason),
		}
	}
	cp.Content = disc
	return cp
}

// disconnectForTopicNameInvalid 构造 Topic Name Invalid 的 DISCONNECT 包。
func disconnectForTopicNameInvalid(reason string) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	disc := &packets.Disconnect{
		ReasonCode: packets.DisconnectTopicNameInvalid,
	}
	if reason != "" {
		disc.Properties = &packets.DisconnectProperties{
			ReasonString: fmt.Sprintf("topic name invalid: %s", reason),
		}
	}
	cp.Content = disc
	return cp
}

// disconnectForRetainNotSupported 构造 Retain Not Supported 的 DISCONNECT 包。
func disconnectForRetainNotSupported(reason string) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	disc := &packets.Disconnect{
		ReasonCode: packets.DisconnectRetainNotSupported,
	}
	if reason != "" {
		disc.Properties = &packets.DisconnectProperties{
			ReasonString: fmt.Sprintf("retain not supported: %s", reason),
		}
	}
	cp.Content = disc
	return cp
}

// disconnectForQoSNotSupported 构造 QoS Not Supported 的 DISCONNECT 包。
func disconnectForQoSNotSupported(reason string) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	disc := &packets.Disconnect{
		ReasonCode: packets.DisconnectQoSNotSupported,
	}
	if reason != "" {
		disc.Properties = &packets.DisconnectProperties{
			ReasonString: fmt.Sprintf("qos not supported: %s", reason),
		}
	}
	cp.Content = disc
	return cp
}
