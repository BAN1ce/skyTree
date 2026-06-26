package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// handleAuth 处理连接建立中或已连接后的 MQTT5 AUTH 包。
//
// 建连阶段的 AUTH 会继续推进 pending CONNECT；已连接后的 AUTH 用于重新认证交互。
func (i *InnerHandler) handleAuth(ctx context.Context, authPacket *packets.Auth) error {
	clientID := i.client.getID()
	startTime := time.Now()

	// AUTH 包必须带上协商一致的 Authentication Method，否则立即断开。
	if authPacket == nil || authPacket.Properties == nil || authPacket.Properties.AuthMethod == "" {
		return i.failProtocol("AUTH packet requires Authentication Method")
	}
	if i.client.enhancedAuthMethod == "" {
		return i.failProtocol("AUTH packet received without negotiated enhanced authentication")
	}
	if authPacket.Properties.AuthMethod != i.client.enhancedAuthMethod {
		return i.failProtocol("AUTH method does not match negotiated method")
	}

	switch i.client.enhancedAuthState {
	case enhancedAuthAuthenticating:
		if authPacket.ReasonCode != packets.AuthContinueAuthentication {
			return i.failProtocol("initial enhanced authentication AUTH must use Continue Authentication")
		}
		return i.handleAuthDuringConnect(ctx, authPacket, startTime)
	case enhancedAuthAuthenticated:
		if authPacket.ReasonCode != packets.AuthReauthenticate {
			return i.failProtocol("connected enhanced authentication AUTH must start with Re-authenticate")
		}
		return i.handleAuthAfterConnect(ctx, clientID, authPacket, startTime)
	case enhancedAuthReauthenticating:
		if authPacket.ReasonCode != packets.AuthContinueAuthentication {
			return i.failProtocol("continued enhanced re-authentication AUTH must use Continue Authentication")
		}
		return i.handleAuthAfterConnect(ctx, clientID, authPacket, startTime)
	default:
		return i.failProtocol("AUTH packet received without negotiated enhanced authentication")
	}
}

// handleAuthAfterConnect 处理已建立连接后的增强认证交互流程。
func (i *InnerHandler) handleAuthAfterConnect(ctx context.Context, clientID string, authPacket *packets.Auth, startTime time.Time) error {
	authResp, err := i.receiveAuthResponse(ctx, clientID, authPacket)
	if err != nil {
		return i.failProtocol("enhanced re-authentication failed")
	}
	normalizeAuthResponse(authResp, i.client.enhancedAuthMethod)
	metric.RecordAuthDuration("plugin", time.Since(startTime))
	switch authResp.ReasonCode {
	case packets.AuthContinueAuthentication:
		i.client.enhancedAuthState = enhancedAuthReauthenticating
	case packets.AuthSuccess:
		i.client.enhancedAuthState = enhancedAuthAuthenticated
	default:
		return i.failProtocol("enhanced re-authentication returned invalid reason code")
	}
	return i.sendAuthPacket(ctx, authResp)
}

// handleAuthDuringConnect 继续处理 CONNECT 被暂停后的增强认证多轮握手。
func (i *InnerHandler) handleAuthDuringConnect(ctx context.Context, authPacket *packets.Auth, startTime time.Time) error {
	authResp, err := i.receiveAuthResponse(ctx, i.client.getID(), authPacket)
	if err != nil {
		return i.failEnhancedAuthConnect(authConnectFailureCode(err), "enhanced authentication failed: "+err.Error(), err)
	}
	normalizeAuthResponse(authResp, i.client.enhancedAuthMethod)
	metric.RecordAuthDuration("plugin", time.Since(startTime))

	// 继续认证则再发 AUTH；认证成功后恢复 pending CONNECT 并进入 completeConnect。
	switch authResp.ReasonCode {
	case packets.AuthContinueAuthentication:
		return i.sendAuthPacket(ctx, authResp)
	case packets.AuthSuccess:
		pendingConnect := i.client.pendingEnhancedAuthConnect
		if pendingConnect == nil {
			return i.failEnhancedAuthConnect(
				packets.ConnAckProtocolError,
				"missing pending CONNECT for enhanced authentication",
				ErrProtocolError,
			)
		}
		assignedByServer := i.client.pendingEnhancedAuthAssignedByServer
		i.client.pendingEnhancedAuthConnect = nil
		i.client.pendingEnhancedAuthAssignedByServer = false
		i.client.enhancedAuthState = enhancedAuthAuthenticated
		i.client.enhancedAuthData = cloneBytes(authResp.Properties.AuthData)

		conAck := packets.NewControlPacket(packets.CONNACK)
		conAckContent := &packets.ConnAck{}
		conAck.Content = conAckContent
		return i.completeConnect(pendingConnect, conAck, conAckContent, assignedByServer)
	default:
		reason := fmt.Sprintf("enhanced authentication rejected with reason code 0x%X", authResp.ReasonCode)
		if authResp.Properties != nil && authResp.Properties.ReasonString != "" {
			reason = authResp.Properties.ReasonString
		}
		return i.failEnhancedAuthConnect(packets.ConnAckNotAuthorized, reason, ErrProtocolError)
	}
}

// failEnhancedAuthConnect 在建连阶段增强认证失败时统一返回 CONNACK 失败并关闭连接。
func (i *InnerHandler) failEnhancedAuthConnect(reasonCode byte, reason string, cause error) error {
	if i == nil || i.client == nil {
		if cause != nil {
			return cause
		}
		return ErrProtocolError
	}
	_ = i.client.failConnect(reasonCode, reason)
	_ = i.client.close()
	if cause != nil {
		return cause
	}
	return ErrProtocolError
}

// authConnectFailureCode 将认证错误映射到 CONNECT 阶段可用的 ReasonCode。
func authConnectFailureCode(err error) byte {
	if errors.Is(err, ErrAuthHandlerNotSet) {
		return packets.ConnAckBadAuthenticationMethod
	}
	return packets.ConnAckNotAuthorized
}

// cloneBytes 深拷贝字节切片，避免插件返回的 AuthData 被后续调用方修改。
func cloneBytes(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

// handleDisconnect 处理客户端主动 DISCONNECT。
//
// 主动断开会标记不发送遗嘱，并允许 MQTT5 客户端在合法情况下覆盖 Session Expiry Interval。
func (i *InnerHandler) handleDisconnect(ctx context.Context, disconnectPacket *packets.Disconnect) error {
	if disconnectPacket == nil {
		disconnectPacket = &packets.Disconnect{ReasonCode: packets.DisconnectNormalDisconnection}
	}
	if !isClientDisconnectReasonCodeAllowed(disconnectPacket.ReasonCode) {
		return i.failProtocol(fmt.Sprintf("DISCONNECT reason code 0x%X is not valid from client", disconnectPacket.ReasonCode))
	}

	logger.Logger.Info().Str("client", i.client.getID()).Msg("delete will message for disconnecting")
	if disconnectPacket.Properties != nil && disconnectPacket.Properties.ServerReference != "" {
		return i.failProtocol("Server Reference is not allowed in client DISCONNECT")
	}
	if i.client.component != nil && i.client.component.plugin != nil {
		if err := i.client.component.plugin.DoReceivedDisconnect(ctx, i.client.getID(), disconnectPacket); err != nil {
			logger.Logger.Warn().Err(err).Str("client", i.client.getID()).Msg("plugin DoReceivedDisconnect error")
		}
	}
	// MQTT5 规定 CONNECT 中会话过期为 0 时，DISCONNECT 不能再把它改成非 0。
	if disconnectPacket.Properties != nil && disconnectPacket.Properties.SessionExpiryInterval != nil {
		sessionExpiryInterval := *disconnectPacket.Properties.SessionExpiryInterval
		if i.client.connectSessionExpiryInterval == 0 && sessionExpiryInterval != 0 {
			return i.failProtocol("DISCONNECT Session Expiry Interval is not allowed when CONNECT Session Expiry Interval is 0")
		}
		i.client.sessionExpiryInterval = capSessionExpiryInterval(sessionExpiryInterval, i.client.brokerRuntimeConfig().Limits)
		i.client.cleanSession = i.client.sessionExpiryInterval == 0
	}
	i.client.disconnectWithoutWill.Store(int64(disconnectPacket.ReasonCode))

	return i.client.close()
}

// isClientDisconnectReasonCodeAllowed 按 MQTT5 DISCONNECT reason code 的 sender 语义
// 校验客户端上行是否允许使用该原因码。
func isClientDisconnectReasonCodeAllowed(code byte) bool {
	switch code {
	case packets.DisconnectNormalDisconnection,
		packets.DisconnectDisconnectWithWillMessage,
		packets.DisconnectUnspecifiedError,
		packets.DisconnectMalformedPacket,
		packets.DisconnectProtocolError,
		packets.DisconnectImplementationSpecificError,
		packets.DisconnectBadAuthenticationMethod,
		packets.DisconnectTopicNameInvalid,
		packets.DisconnectReceiveMaximumExceeded,
		packets.DisconnectTopicAliasInvalid,
		packets.DisconnectPacketTooLarge,
		packets.DisconnectMessageRateTooHigh,
		packets.DisconnectQuotaExceeded,
		packets.DisconnectAdministrativeAction,
		packets.DisconnectPayloadFormatInvalid:
		return true
	default:
		return false
	}
}

// mqtt5ConnectSessionExpiryInterval 读取 CONNECT 携带的 Session Expiry Interval 原始值。
func mqtt5ConnectSessionExpiryInterval(connectPacket *packets.Connect) uint32 {
	if connectPacket == nil || connectPacket.Properties == nil || connectPacket.Properties.SessionExpiryInterval == nil {
		return 0
	}
	return *connectPacket.Properties.SessionExpiryInterval
}

// mqtt5SessionExpiryInterval 计算 CONNECT 协商后的会话过期时间：
// - 客户端未携带该属性时按 MQTT5 默认值 0；
// - 服务端上限由 broker.limits.session_expiry_max_seconds 控制（0=不限）。
func mqtt5SessionExpiryInterval(connectPacket *packets.Connect, limits config.BrokerLimits) uint32 {
	return capSessionExpiryInterval(mqtt5ConnectSessionExpiryInterval(connectPacket), limits)
}

// mqtt5KeepAlive 计算最终 KeepAlive：服务端配置可覆盖客户端 CONNECT 中的值。
func mqtt5KeepAlive(connectPacket *packets.Connect, prop *config.ConnectAckProperty) time.Duration {
	keepAliveSeconds := 0
	if connectPacket != nil {
		keepAliveSeconds = int(connectPacket.KeepAlive)
	}
	if prop != nil && prop.ServerKeepAlive >= 0 {
		keepAliveSeconds = clampInt(prop.ServerKeepAlive, 0, maxUint16)
	}
	return time.Duration(keepAliveSeconds) * time.Second
}

// serverReceiveMaximum 返回 broker 允许的上行 QoS1/QoS2 inflight 最大值。
func serverReceiveMaximum(brokerCfg config.Broker) uint16 {
	if v := brokerCfg.ConnectAckProperty.ReceiveMaximum; v > 0 {
		if v > 65535 {
			return 65535
		}
		return uint16(v)
	}
	return 65535
}

// mqtt5RequestProblemInfo 判断客户端是否请求携带 MQTT5 Problem Information。
func mqtt5RequestProblemInfo(connectPacket *packets.Connect) bool {
	if connectPacket == nil || connectPacket.Properties == nil || connectPacket.Properties.RequestProblemInfo == nil {
		return true
	}
	return *connectPacket.Properties.RequestProblemInfo != 0
}

// UpdateClientAliveTime refreshes the in-memory alive time and local expiration index.
func (i *InnerHandler) UpdateClientAliveTime() {
	if i == nil || i.client == nil {
		return
	}

	now := time.Now()
	i.client.aliveTime.Store(now)
	if i.client.component == nil || i.client.component.keepAliveTracker == nil {
		return
	}
	if i.client.getID() == "" {
		return
	}
	ownerToken := i.client.getOwnerToken()
	if ownerToken == "" {
		logger.Logger.Debug().Str("client", i.client.getID()).Msg("skip keepalive tracker update without owner token")
		return
	}

	i.client.component.keepAliveTracker.Update(i.client.getID(), ownerToken, now, i.client.GetKeepAliveTime())
}
