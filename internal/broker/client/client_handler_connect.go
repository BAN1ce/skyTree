package client

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
)

// applyConnectClientPreferences 应用客户端在 CONNECT 中声明的协商偏好。
func (i *InnerHandler) applyConnectClientPreferences(connectPacket *packets.Connect) {
	if i == nil || i.client == nil {
		return
	}
	i.client.requestProblemInfo = mqtt5RequestProblemInfo(connectPacket)
	if connectPacket == nil || connectPacket.Properties == nil || connectPacket.Properties.MaximumPacketSize == nil {
		return
	}
	clientMaxSize := *connectPacket.Properties.MaximumPacketSize
	if clientMaxSize == 0 {
		return
	}
	i.client.clientMaximumPacketSize = &clientMaxSize
}

// setupPublishBucket 根据 CONNECT 中的 Receive Maximum 初始化下行发布限流桶。
//
// 优先级：CONNECT.ReceiveMaximum > MQTT5 默认值 65535 >
// config.WindowSize > config.ClientRateLimit.WindowSize > 最小值 1。
func (i *InnerHandler) setupPublishBucket(connectPacket *packets.Connect) {
	if i.client == nil {
		logger.Logger.Error().Msg("client is nil, setupPublishBucket")
		return
	}

	var windowSize int

	// 优先使用 CONNECT 中的 ReceiveMaximum。
	if connectPacket.Properties != nil && connectPacket.Properties.ReceiveMaximum != nil {
		windowSize = int(*connectPacket.Properties.ReceiveMaximum)
	} else {
		// MQTT 5.0 未提供 ReceiveMaximum 时默认 65535；兼容无 Properties 的客户端。
		windowSize = 65535
	}

	// 如果协议值无效，再回退到本地配置中的窗口大小。
	if windowSize <= 0 && i.client.component != nil {
		if i.client.component.cfg.WindowSize > 0 {
			windowSize = i.client.component.cfg.WindowSize
		} else if i.client.component.cfg.BrokerConfig.ClientRateLimit.WindowSize > 0 {
			windowSize = i.client.component.cfg.BrokerConfig.ClientRateLimit.WindowSize
		}
	}

	// 最小窗口为 1，保证限流机制始终可用。
	if windowSize <= 0 {
		windowSize = 1
	}
	i.client.publishBucket = rate.NewBucket(windowSize)
}

// setupIncomingPublishRateLimiter 按当前 broker 配置初始化上行 PUBLISH 速率限制器。
func (i *InnerHandler) setupIncomingPublishRateLimiter() {
	if i == nil || i.client == nil {
		return
	}
	cfg := i.client.brokerRuntimeConfig().ClientRateLimit
	if !cfg.Enabled || cfg.MessagesPerSecond <= 0 {
		i.client.incomingPublishRateLimiter = nil
		return
	}
	i.client.incomingPublishRateLimiter = newIncomingPublishRateLimiter(cfg.MessagesPerSecond, time.Second)
}

// handleConnect 处理客户端的初始 CONNECT 包。
//
// 它负责基础协议校验、ClientID 确定、上下文绑定和增强认证首轮处理；
// 如果增强认证需要多轮交互，则暂存 CONNECT，等待 AUTH 包继续完成连接。
func (i *InnerHandler) handleConnect(connectPacket *packets.Connect) error {
	var (
		conAck           = packets.NewControlPacket(packets.CONNACK)
		assignedByServer bool
	)

	conAckContent := &packets.ConnAck{}
	conAck.Content = conAckContent

	// 在协议校验前先应用客户端协商偏好（如 Maximum Packet Size），
	// 以确保失败 CONNACK 也受客户端上限约束。
	i.applyConnectClientPreferences(connectPacket)

	// 先做不依赖会话中心的协议合法性校验，失败时直接返回 CONNACK 错误。
	if err := i.validConnect(connectPacket); err != nil {
		i.logConnectFailure(connectPacket, "valid_connect", err)
		return err
	}

	// ClientID 为空时由服务端分配，并记录到 context，供后续插件和存储链路使用。
	i.client.ID = connectPacket.ClientID
	logger.Logger.Debug().Str("client id ", connectPacket.ClientID).Msg("client connected")
	if i.client.ID == "" {
		assignedByServer = true
		i.client.ID = uuid.NewString()
	}

	i.client.ctx = pkg.SetClientID(i.client.ctx, i.client.ID)

	// MQTT5 增强认证可能要求先返回 AUTH，再等客户端继续 AUTH 后才发送 CONNACK。
	waitingForAuth, err := i.handleInitialEnhancedAuth(i.client.getCtx(), connectPacket, conAck, conAckContent, assignedByServer)
	if err != nil || waitingForAuth {
		if err != nil {
			i.logConnectFailure(connectPacket, "initial_enhanced_auth", err)
		}
		return err
	}
	return i.completeConnect(connectPacket, conAck, conAckContent, assignedByServer)
}

// completeConnect 在 CONNECT 校验和可能的增强认证完成后正式建立会话。
//
// 该流程会打开/接管会话、清理旧 owner、恢复订阅和 inflight 状态、写回 CONNACK，
// 最后启动客户端维度的投递 runner。
func (i *InnerHandler) completeConnect(connectPacket *packets.Connect, conAck *packets.ControlPacket, conAckContent *packets.ConnAck, assignedByServer bool) error {
	if err := i.handleConnectAdmission(connectPacket, conAck, conAckContent); err != nil {
		i.logConnectFailure(connectPacket, "connect_admission", err)
		return err
	}
	if err := i.runConnectPlugin(connectPacket, conAck, conAckContent); err != nil {
		i.logConnectFailure(connectPacket, "connect_plugin", err)
		return err
	}

	setup := i.initializeConnectSetup(connectPacket)
	sessionResp, err := i.acquireSessionForConnect(connectPacket, setup)
	if err != nil {
		i.logConnectFailure(connectPacket, "acquire_session", err)
		return err
	}
	if err := i.applyTakenOverOwnerToken(sessionResp.Takeover); err != nil {
		i.logConnectFailure(connectPacket, "apply_owner_token", err)
		return err
	}
	i.closePreviousSessionOwner(sessionResp.Takeover, sessionResp.OpenSession)
	i.deletePendingWillDelayTask()
	i.cleanupCleanStartState(connectPacket)
	i.registerClientConnection()
	i.prepareConnectedClientRuntime(connectPacket)
	i.setupPublishBucket(connectPacket)
	i.setupIncomingPublishRateLimiter()
	i.prepareClientWillMessage(connectPacket)
	i.prepareSuccessConnAck(connectPacket, conAckContent, assignedByServer, sessionResp.OpenSession, setup.sessionExpiryInterval)

	if err := i.client.write(&clientcap.WritePacket{
		Packet: conAck,
	}); err != nil {
		i.logConnectFailure(connectPacket, "write_connack", err)
		return err
	}
	i.logConnectSuccess(connectPacket, sessionResp.OpenSession)
	i.UpdateClientAliveTime()
	i.restoreSessionAfterConnAck(connectPacket, sessionResp.OpenSession)
	i.client.StartClientDeliveryRunner()
	return nil
}

func (i *InnerHandler) logConnectFailure(connectPacket *packets.Connect, stage string, err error) {
	if i == nil || i.client == nil || err == nil {
		return
	}
	logger.Logger.Warn().
		Err(err).
		Str("stage", stage).
		Uint64("node_id", i.client.clusterRuntimeConfig().LocalNodeID).
		Bool("clean_start", connectPacket != nil && connectPacket.CleanStart).
		Bool("has_will", connectPacket != nil && connectPacket.WillFlag).
		Bool("connack_accepted", i.client.connAckAccepted.Load()).
		Msg("connect failed before completion")
}

func (i *InnerHandler) logConnectSuccess(connectPacket *packets.Connect, openResp *proto_session.OpenSessionForConnectResponse) {
	if i == nil || i.client == nil {
		return
	}
	oldSessionExists := false
	sessionExists := false
	if openResp != nil {
		oldSessionExists = openResp.GetOldSessionExists()
		sessionExists = openResp.GetSession() != nil
	}
	logger.Logger.Debug().
		Uint64("node_id", i.client.clusterRuntimeConfig().LocalNodeID).
		Bool("clean_start", connectPacket != nil && connectPacket.CleanStart).
		Bool("has_will", connectPacket != nil && connectPacket.WillFlag).
		Bool("old_session_exists", oldSessionExists).
		Bool("session_exists", sessionExists).
		Msg("connect completed and connack written")
}

type connectSetup struct {
	ownerToken                   string
	willMessage                  *proto_session.WillMessage
	sessionExpiryInterval        uint32
	connectSessionExpiryInterval uint32
}

// runConnectPlugin 执行 CONNECT 插件钩子，并在拒绝时回写 CONNACK。
func (i *InnerHandler) runConnectPlugin(connectPacket *packets.Connect, conAck *packets.ControlPacket, conAckContent *packets.ConnAck) error {
	if i.client.component == nil || i.client.component.plugin == nil {
		return nil
	}
	// mTLS：把客户端证书链塞进 ctx，让插件能基于证书做 ACL / 用户名映射。
	pluginCtx := withPeerCertificates(i.client.getCtx(), peerCertificatesFromConn(i.client.GetConn()))
	if err := i.client.component.plugin.DoReceivedConnect(pluginCtx, i.client.ID, connectPacket); err != nil {
		// 把插件返回的错误透传到 ReasonString，spec §3.1.2.11.7 允许 CONNACK 始终携带。
		conAckContent.ReasonCode = packets.ConnAckNotAuthorized
		conAckContent.Properties = ensureConnAckProperties(conAckContent.Properties)
		conAckContent.Properties.ReasonString = err.Error()
		_ = i.client.write(&clientcap.WritePacket{Packet: conAck})
		_ = i.client.close()
		return err
	}
	return nil
}

// initializeConnectSetup 初始化本次连接建立流程中需要的会话参数。
func (i *InnerHandler) initializeConnectSetup(connectPacket *packets.Connect) connectSetup {
	// 集群环境下关闭旧连接是异步副作用，可能延迟或乱序到达；
	// OwnerToken 同时写入 sessionCenter 和内存 Client，远端 CloseClient 必须携带匹配 token。
	setup := connectSetup{
		ownerToken:                   uuid.NewString(),
		willMessage:                  session.WillMessageToProto(connectPacket),
		sessionExpiryInterval:        mqtt5SessionExpiryInterval(connectPacket, i.client.brokerRuntimeConfig().Limits),
		connectSessionExpiryInterval: mqtt5ConnectSessionExpiryInterval(connectPacket),
	}
	brokerCfg := i.client.brokerRuntimeConfig()
	i.client.sessionExpiryInterval = setup.sessionExpiryInterval
	i.client.connectSessionExpiryInterval = setup.connectSessionExpiryInterval
	i.client.cleanSession = setup.sessionExpiryInterval == 0
	i.client.serverReceiveMaximum = serverReceiveMaximum(brokerCfg)
	i.client.enhancedAuthMethod = ""
	if connectPacket.Properties != nil {
		i.client.enhancedAuthMethod = connectPacket.Properties.AuthMethod
	}
	return setup
}

// acquireSessionForConnect 通过 StateRouter 完成会话打开、clean start 重置和 owner 接管。
func (i *InnerHandler) acquireSessionForConnect(
	connectPacket *packets.Connect,
	setup connectSetup,
) (*staterouter.AcquireSessionResponse, error) {
	if i.client.component == nil || i.client.component.stateRouter == nil {
		return nil, fmt.Errorf("state router is nil")
	}
	return i.client.component.stateRouter.AcquireSession(i.client.getCtx(), staterouter.AcquireSessionRequest{
		BrokerNodeID:          i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:              i.client.ID,
		OwnerToken:            setup.ownerToken,
		WillMessage:           setup.willMessage,
		SessionExpiryInterval: setup.sessionExpiryInterval,
		CleanStart:            connectPacket.CleanStart,
		NowUnixNano:           time.Now().UnixNano(),
	})
}

// applyTakenOverOwnerToken 应用 StateRouter 返回的新 owner token。
func (i *InnerHandler) applyTakenOverOwnerToken(takeOverResp *proto_session.TakeOverSessionOwnerResponse) error {
	if takeOverResp == nil || takeOverResp.GetOwner() == nil || takeOverResp.GetOwner().GetOwnerToken() == "" {
		return fmt.Errorf("state router takeover response missing owner token")
	}
	i.client.setOwnerToken(takeOverResp.GetOwner().GetOwnerToken())
	return nil
}

// closePreviousSessionOwner 尝试关闭被抢占的旧连接 owner。
func (i *InnerHandler) closePreviousSessionOwner(
	takeOverResp *proto_session.TakeOverSessionOwnerResponse,
	openResp *proto_session.OpenSessionForConnectResponse,
) {
	if takeOverResp != nil && takeOverResp.GetPreviousOwnerExists() && takeOverResp.GetPreviousOwner() != nil {
		prevOwner := takeOverResp.GetPreviousOwner()
		logger.Logger.Info().Uint64("node", prevOwner.GetNodeID()).Str("id", i.client.ID).Msg("client exist in other node maybe")
		if err := i.client.component.stateRouter.ClosePreviousOwner(i.client.getCtx(), staterouter.ClosePreviousOwnerRequest{
			BrokerNodeID:  i.client.clusterRuntimeConfig().LocalNodeID,
			ClientID:      i.client.getID(),
			PreviousOwner: prevOwner,
		}); err != nil {
			logger.Logger.Error().Err(err).Str("client", i.client.metaString()).Msg("close old client error")
		}
		return
	}
	if openResp != nil {
		logger.Logger.Debug().Bool("exists", openResp.GetOldSessionExists()).Msg("old persistent session exists")
	}
}

// deletePendingWillDelayTask 删除历史异常断开残留的延迟遗嘱任务。
func (i *InnerHandler) deletePendingWillDelayTask() {
	// 客户端重连成功后，之前异常断开遗留的延迟遗嘱任务必须取消。
	if i.client.component.willDelayCenter == nil {
		return
	}
	if err := i.client.component.willDelayCenter.DeleteTask(i.client.getCtx(), i.client.getID()); err != nil {
		logger.Logger.Warn().
			Err(err).
			Str("client", i.client.getID()).
			Msg("failed to delete pending will delay task on reconnect")
		return
	}
	logger.Logger.Debug().
		Str("client", i.client.getID()).
		Msg("deleted pending will delay task on reconnect")
}

// cleanupCleanStartState 在 Clean Start 时清理旧订阅和离线投递状态。
func (i *InnerHandler) cleanupCleanStartState(connectPacket *packets.Connect) {
	if !connectPacket.CleanStart {
		return
	}
	// CleanStart 删除订阅时也带 OwnerToken，避免延迟请求误删新 owner 的订阅。
	if err := i.client.component.stateRouter.DeleteClientSubscriptions(i.client.getCtx(), staterouter.DeleteClientSubscriptionsRequest{
		BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:     i.client.getID(),
		OwnerToken:   i.client.getOwnerToken(),
	}); err != nil {
		logger.Logger.Error().Err(err).Str("client", i.client.metaString()).Msg("failed to delete subscriptions for clean start")
	}
	i.client.deleteClientDeliveryState(i.client.getCtx(), "clean start")
}

// registerClientConnection 将当前连接注册到本地 client manager。
func (i *InnerHandler) registerClientConnection() {
	// 注册到本节点连接管理器；同节点已有旧连接时立即关闭旧连接。
	oldClient := i.client.component.clientManager.AddClient(i.client.getID(), i.client)
	if oldClient == nil || oldClient == i.client {
		return
	}
	logger.Logger.Info().Str("client id", i.client.ID).Msg("client exist, close old client")
	// 保持和 Close 一致的加锁语义：先锁旧 client，再关闭。
	oldClient.mux.Lock()
	err := oldClient.closeWithSessionTakenOverLocked()
	oldClient.mux.Unlock()
	if err != nil {
		logger.Logger.Error().Err(err).Str("client", i.client.metaString()).Msg("fail to close old client")
	}
}

// prepareConnectedClientRuntime 绑定连接建立后的运行时字段。
func (i *InnerHandler) prepareConnectedClientRuntime(connectPacket *packets.Connect) {
	brokerCfg := i.client.brokerRuntimeConfig()
	i.client.keepAlive = mqtt5KeepAlive(connectPacket, &brokerCfg.ConnectAckProperty)
	i.client.Username = connectPacket.Username
	// CleanStart=false 时恢复共享订阅的在线关系。
	// 非 CleanStart 重连需要把共享订阅重新挂到本节点的共享订阅 manager。
	if !connectPacket.CleanStart && i.client.component != nil && i.client.component.sharedSubscriptionManager != nil && i.client.component.subCenter != nil {
		go i.restoreSharedSubscriptions(i.client.getCtx())
	}
}

// prepareClientWillMessage 构造并缓存客户端遗嘱消息。
func (i *InnerHandler) prepareClientWillMessage(connectPacket *packets.Connect) {
	// 解析并暂存遗嘱消息；真正投递由连接异常关闭流程决定。
	if !connectPacket.WillFlag {
		return
	}
	willMessage := packets.NewControlPacket(packets.PUBLISH)
	willPublish := &packets.Publish{
		QoS:     connectPacket.WillQOS,
		Retain:  connectPacket.WillRetain,
		Topic:   connectPacket.WillTopic,
		Payload: connectPacket.WillMessage,
	}
	willPublish.Properties = publishPropertiesFromWill(connectPacket.WillProperties)
	willMessage.Content = willPublish
	i.client.willMessage = &brokerpublish.Message{
		SendClientID: i.client.getID(),
	}
	if connectPacket.WillProperties != nil && connectPacket.WillProperties.WillDelayInterval != nil {
		i.client.willMessage.WillDelay = time.Duration(*connectPacket.WillProperties.WillDelayInterval) * time.Second
	}
	i.client.willMessage.SetControlPacket(willMessage)
}

// publishPropertiesFromWill 复制遗嘱属性到标准 PUBLISH 属性结构。
func publishPropertiesFromWill(willProperties *packets.PublishProperties) *packets.PublishProperties {
	if willProperties == nil {
		return nil
	}
	properties := &packets.PublishProperties{
		PayloadFormat: willProperties.PayloadFormat,
		ContentType:   willProperties.ContentType,
		ResponseTopic: willProperties.ResponseTopic,
		MessageExpiry: willProperties.MessageExpiry,
	}
	if len(willProperties.CorrelationData) > 0 {
		properties.CorrelationData = cloneBytes(willProperties.CorrelationData)
	}
	if len(willProperties.User) > 0 {
		properties.User = make([]packets.User, len(willProperties.User))
		copy(properties.User, willProperties.User)
	}
	return properties
}

// prepareSuccessConnAck 组装连接成功时返回给客户端的 CONNACK。
func (i *InnerHandler) prepareSuccessConnAck(
	connectPacket *packets.Connect,
	conAckContent *packets.ConnAck,
	assignedByServer bool,
	openResp *proto_session.OpenSessionForConnectResponse,
	sessionExpiryInterval uint32,
) {
	// CleanStart=false 且服务端存在旧会话时，SessionPresent=true 告知客户端可继续使用旧状态。
	if !connectPacket.CleanStart && openResp != nil && openResp.GetOldSessionExists() {
		conAckContent.SessionPresent = true
	}
	brokerCfg := i.client.brokerRuntimeConfig()
	conAckContent.ReasonCode = packets.ConnAckSuccess
	applyAssignedClientIDToConnAck(conAckContent, assignedByServer, i.client.ID)
	i.resetDownlinkTopicAlias(connectPacket)
	applyConnectAwareConfigToConnAckProperties(conAckContent, &brokerCfg.ConnectAckProperty, connectPacket)
	applyRuntimeCapabilitiesToConnAckProperties(conAckContent, i.client.component)
	applyNegotiatedSessionExpiryToConnAckProperties(conAckContent, sessionExpiryInterval)
	applyEnhancedAuthToConnAckProperties(conAckContent, i.client.enhancedAuthMethod, i.client.enhancedAuthData)
}

// resetDownlinkTopicAlias 根据 CONNECT 协商重置下行 Topic Alias 状态。
func (i *InnerHandler) resetDownlinkTopicAlias(connectPacket *packets.Connect) {
	// CONNECT.TopicAliasMaximum 表示客户端能接受的下行别名数；
	// CONNACK.TopicAliasMaximum 表示服务端接受的上行别名数。
	i.client.writeMux.Lock()
	defer i.client.writeMux.Unlock()
	if i.client.topicAliasManager == nil {
		return
	}
	i.client.topicAliasManager.ResetDownlink()
	if connectPacket.Properties != nil && connectPacket.Properties.TopicAliasMaximum != nil {
		i.client.topicAliasManager.SetDownlinkMax(*connectPacket.Properties.TopicAliasMaximum)
		return
	}
	i.client.topicAliasManager.SetDownlinkMax(0)
}

// restoreSessionAfterConnAck 在非 Clean Start 下恢复 inflight 会话状态。
func (i *InnerHandler) restoreSessionAfterConnAck(connectPacket *packets.Connect, openResp *proto_session.OpenSessionForConnectResponse) {
	// CleanStart=false 时恢复下行 QoS1/QoS2 inflight；放在 CONNACK 后、runner 前，避免使用新 PacketID 重发。
	if connectPacket.CleanStart || openResp == nil || openResp.GetSession() == nil {
		return
	}
	i.client.restoreIncomingQoS2FromSession(i.client.getCtx(), openResp.GetSession())
	i.client.restoreOutgoingInflightFromSession(i.client.getCtx(), openResp.GetSession())
}

// handleConnectAdmission 运行连接准入回调并处理拒绝响应。
func (i *InnerHandler) handleConnectAdmission(
	connectPacket *packets.Connect,
	conAck *packets.ControlPacket,
	conAckContent *packets.ConnAck,
) error {
	if i == nil || i.client == nil || i.client.component == nil || i.client.component.connectAdmission == nil {
		return nil
	}
	result := i.client.component.connectAdmission(i.client.getCtx(), i.client.getID(), connectPacket)
	if result.ReasonCode == 0 || result.ReasonCode == packets.ConnAckSuccess {
		return nil
	}
	conAckContent.ReasonCode = result.ReasonCode
	conAckContent.Properties = ensureConnAckProperties(conAckContent.Properties)
	if result.ReasonString != "" {
		conAckContent.Properties.ReasonString = result.ReasonString
	}
	if connAckUsesServerReference(result.ReasonCode) {
		conAckContent.Properties.ServerReference = result.ServerReference
		if conAckContent.Properties.ServerReference == "" {
			conAckContent.Properties.ServerReference = i.client.brokerRuntimeConfig().ConnectAckProperty.ServerReference
		}
	}
	_ = i.client.write(&clientcap.WritePacket{Packet: conAck})
	_ = i.client.close()
	return ErrProtocolError
}

// validConnect 校验 CONNECT 包中可以在建立会话前确定的协议约束。
//
// 校验失败时会直接写回对应 ReasonCode 的 CONNACK，调用方只需返回错误。
func (i *InnerHandler) validConnect(connectPacket *packets.Connect) error {
	if err := i.validateConnectBasics(connectPacket); err != nil {
		return err
	}
	if err := i.validateConnectProperties(connectPacket); err != nil {
		return err
	}
	if err := i.validateConnectWill(connectPacket); err != nil {
		return err
	}
	return i.validateConnectConfiguredLimits(connectPacket)
}

// validateConnectBasics 校验协议版本与用户名密码标志等基础约束。
func (i *InnerHandler) validateConnectBasics(connectPacket *packets.Connect) error {
	// ClientID 为空且 CleanStart=false 时，服务端无法定位旧会话，直接拒绝。
	if connectPacket.ClientID == "" && !connectPacket.CleanStart {
		_ = i.client.failConnect(packets.ConnAckInvalidClientID, "client id is empty and clean start is false")
		return ErrClientIDEmpty
	}

	// 当前 broker 只接受 MQTT 5.0，其他协议版本立即拒绝。
	if connectPacket.ProtocolName != "MQTT" || connectPacket.ProtocolVersion != 0x05 {
		_ = i.client.failConnect(
			packets.ConnAckUnsupportedProtocolVersion,
			fmt.Sprintf("unsupported protocol %q version %d", connectPacket.ProtocolName, connectPacket.ProtocolVersion),
		)
		return fmt.Errorf("unsupported protocol version %s %d", connectPacket.ProtocolName, connectPacket.ProtocolVersion)
	}

	if !connectPacket.UsernameFlag && len(connectPacket.Username) != 0 {
		_ = i.client.failConnect(packets.ConnAckBadUsernameOrPassword, "username present but username flag is unset")
		return ErrClientUsernameNotEmpty
	}

	if !connectPacket.PasswordFlag && len(connectPacket.Password) != 0 {
		_ = i.client.failConnect(packets.ConnAckBadUsernameOrPassword, "password present but password flag is unset")
		return ErrClientPasswordNotEmpty
	}
	return nil
}

// validateConnectProperties 校验 CONNECT 属性字段的协议合法性。
func (i *InnerHandler) validateConnectProperties(connectPacket *packets.Connect) error {
	if connectPacket.Properties != nil && len(connectPacket.Properties.AuthData) > 0 && connectPacket.Properties.AuthMethod == "" {
		_ = i.client.failConnect(packets.ConnAckBadAuthenticationMethod, "auth data provided without auth method")
		return ErrProtocolError
	}
	// MQTT5 §3.1.2.11.3: ReceiveMaximum=0 is a Protocol Error.
	if connectPacket.Properties != nil && connectPacket.Properties.ReceiveMaximum != nil && *connectPacket.Properties.ReceiveMaximum == 0 {
		_ = i.client.failConnect(packets.ConnAckProtocolError, "receive maximum must not be zero")
		return ErrProtocolError
	}
	// MQTT5 §3.1.2.11.4: MaximumPacketSize=0 is a Protocol Error.
	if connectPacket.Properties != nil && connectPacket.Properties.MaximumPacketSize != nil && *connectPacket.Properties.MaximumPacketSize == 0 {
		_ = i.client.failConnect(packets.ConnAckProtocolError, "maximum packet size must not be zero")
		return ErrProtocolError
	}
	return nil
}

// validateConnectWill 校验遗嘱主题、载荷格式和服务端能力约束。
func (i *InnerHandler) validateConnectWill(connectPacket *packets.Connect) error {
	if !connectPacket.WillFlag {
		return nil
	}
	// 遗嘱 Topic 必须是具体 Topic Name，不能为空，也不能包含通配符。
	if connectPacket.WillTopic == "" || topicutil.HasWildcard(connectPacket.WillTopic) {
		_ = i.client.failConnect(packets.ConnAckTopicNameInvalid, "will topic must be a non-empty concrete topic name")
		return ErrProtocolError
	}
	if connectPacket.WillProperties != nil &&
		connectPacket.WillProperties.PayloadFormat != nil &&
		*connectPacket.WillProperties.PayloadFormat == 1 &&
		!utf8.Valid(connectPacket.WillMessage) {
		_ = i.client.failConnect(packets.ConnAckPayloadFormatInvalid, "will payload is not valid UTF-8 with payload format indicator = 1")
		return ErrProtocolError
	}
	brokerCfg := i.client.brokerRuntimeConfig()
	prop := &brokerCfg.ConnectAckProperty
	if prop.RetainAvailable == 0 && connectPacket.WillRetain {
		_ = i.client.failConnect(packets.ConnAckRetainNotSupported, "retain is not supported by this server")
		return ErrProtocolError
	}
	if int(connectPacket.WillQOS) > prop.MaxQos {
		_ = i.client.failConnect(packets.ConnAckQoSNotSupported, fmt.Sprintf("requested will QoS %d exceeds server maximum %d", connectPacket.WillQOS, prop.MaxQos))
		return ErrProtocolError
	}
	return nil
}

// validateConnectConfiguredLimits 校验 broker 配置定义的 CONNECT 上限。
func (i *InnerHandler) validateConnectConfiguredLimits(connectPacket *packets.Connect) error {
	// MQTT5 §3.1.3.2.2 Will Delay Interval 校验：超过 broker 配置上限直接拒绝。
	limits := i.client.brokerRuntimeConfig().Limits
	if connectPacket.WillFlag && connectPacket.WillProperties != nil &&
		connectPacket.WillProperties.WillDelayInterval != nil && limits.WillDelayMaxSeconds > 0 &&
		*connectPacket.WillProperties.WillDelayInterval > limits.WillDelayMaxSeconds {
		_ = i.client.failConnect(packets.ConnAckProtocolError,
			fmt.Sprintf("will delay interval %d exceeds server maximum %d",
				*connectPacket.WillProperties.WillDelayInterval, limits.WillDelayMaxSeconds))
		return ErrProtocolError
	}
	return nil
}

// capSessionExpiryInterval 按 broker 会话过期上限截断客户端请求值。
func capSessionExpiryInterval(sessionExpiryInterval uint32, limits config.BrokerLimits) uint32 {
	if limits.SessionExpiryMaxSeconds == 0 {
		return sessionExpiryInterval
	}
	if sessionExpiryInterval == mqttNeverExpireSessionInterval {
		return sessionExpiryInterval
	}
	if sessionExpiryInterval > limits.SessionExpiryMaxSeconds {
		return limits.SessionExpiryMaxSeconds
	}
	return sessionExpiryInterval
}
