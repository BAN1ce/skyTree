package client

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetsize"
)

var (
	// 复用同一个 PINGRESP 包，避免每次心跳响应都分配对象。
	pingResp = func() *packets.ControlPacket {
		cp := packets.NewControlPacket(packets.PINGRESP)
		cp.Content = &packets.Pingresp{}
		return cp
	}()
)

// InnerHandler 是单个客户端连接的 MQTT 控制包处理器。
//
// 它持有当前 Client 的运行态，并负责把收到的 CONNECT/PUBLISH/SUBSCRIBE
// 等协议包分发到对应流程中，同时维护会话、鉴权、QoS 握手、限流和保活状态。
type InnerHandler struct {
	client *Client
}

// NewClientHandler 创建绑定到指定 Client 的内部协议处理器。
func NewClientHandler(client *Client) *InnerHandler {
	return &InnerHandler{
		client: client,
	}
}

// HandlePacket 是客户端入站控制包的统一入口。
//
// 该方法在客户端锁内完成协议顺序校验、包大小校验、保活刷新和按包类型分发。
// 注意：这里以及下游 handle* 方法中不要调用会再次加锁的 Client 导出方法。
func (i *InnerHandler) HandlePacket(ctx context.Context, cp *packets.ControlPacket, client *Client) error {
	// 这里以及 handle* 内不要调用会再次加锁的 Client 导出方法，
	// 否则可能因为重复获取 c.mux 造成死锁。
	i.client.mux.Lock()
	defer i.client.mux.Unlock()

	// 收到任意合法控制包都先刷新客户端活跃时间，避免长时间业务处理导致保活状态滞后。
	i.UpdateClientAliveTime()

	if err := i.rejectOversizedInboundPacket(cp); err != nil {
		return err
	}
	if err := i.validateInboundPacketState(cp); err != nil {
		return err
	}
	return i.dispatchInboundPacket(ctx, cp, client)
}

// rejectOversizedInboundPacket 在进入业务处理前统一校验入站包大小上限。
func (i *InnerHandler) rejectOversizedInboundPacket(cp *packets.ControlPacket) error {
	// 使用服务端 Maximum Packet Size 限制检查入站包大小；
	// CONNECT 和非 CONNECT 都在进入具体协议处理前统一拦截。
	serverMaxSize := i.client.brokerRuntimeConfig().ConnectAckProperty.MaximumPacketSize
	if err := packetsize.CheckPacketSizeWithMax(cp, serverMaxSize); err != nil {
		logger.Logger.Warn().
			Err(err).
			Str("client", i.client.metaString()).
			Msg("packet size exceeds server maximum")
		if i.client.canSendDisconnect() {
			packetSize, calcErr := packetsize.GetPacketSize(cp)
			if calcErr != nil {
				packetSize = 0
			}
			_ = i.client.write(&clientcap.WritePacket{
				Packet: disconnectForPacketTooLarge(packetSize, serverMaxSize),
			})
		}
		_ = i.client.close()
		return err
	}
	return nil
}

// validateInboundPacketState 校验会话阶段与报文类型是否匹配。
func (i *InnerHandler) validateInboundPacketState(cp *packets.ControlPacket) error {
	// MQTT 会话建立前只允许 CONNECT；建立后再次 CONNECT 属于协议错误。
	if cp.FixedHeader.Type != packets.CONNECT && i.client.getID() == "" {
		return i.failProtocol("first packet must be CONNECT")
	}
	if cp.FixedHeader.Type == packets.CONNECT && i.client.getID() != "" {
		return i.failProtocol("CONNECT received after session is established")
	}
	// 初始增强认证（CONNECT 尚未完成）期间，只允许 AUTH/DISCONNECT。
	// 已连接后的 re-authentication 阶段仍需继续处理业务包，不能在入口统一拦截。
	if i.client.enhancedAuthState == enhancedAuthAuthenticating {
		if cp.FixedHeader.Type != packets.AUTH && cp.FixedHeader.Type != packets.DISCONNECT {
			return i.failProtocol("only AUTH or DISCONNECT is allowed while enhanced authentication is in progress")
		}
	}
	if !isClientToServerPacket(cp.FixedHeader.Type) {
		return i.failProtocol(fmt.Sprintf("%s is not valid from client to server", packets.PacketTypeName(cp.FixedHeader.Type)))
	}
	return nil
}

// dispatchInboundPacket 根据控制包类型分派到具体处理函数。
func (i *InnerHandler) dispatchInboundPacket(ctx context.Context, cp *packets.ControlPacket, client *Client) error {
	// 按控制包的实际内容类型分发到协议子流程，并在入口处打点。
	switch p := cp.Content.(type) {

	case *packets.Connect:
		metric.RecordMQTTReceivedPacket(packets.CONNECT)
		return i.handleConnect(p)

	case *packets.Publish:
		metric.RecordMQTTPublishPacket(p.QoS, "in")
		metric.RecordMQTTReceivedPacket(packets.PUBLISH)
		return i.handlePublish(ctx, p)

	case *packets.Subscribe:
		metric.RecordMQTTReceivedPacket(packets.SUBSCRIBE)
		return i.handleSub(ctx, p)

	case *packets.Unsubscribe:
		metric.RecordMQTTReceivedPacket(packets.UNSUBSCRIBE)
		return i.handleUnsub(ctx, p)

	case *packets.Puback:
		metric.RecordMQTTReceivedPacket(packets.PUBACK)
		return i.handlePubAck(ctx, p)
	case *packets.Pubrec:
		metric.RecordMQTTReceivedPacket(packets.PUBREC)
		return i.handlePubRec(ctx, p)

	case *packets.Pubrel:
		metric.RecordMQTTReceivedPacket(packets.PUBREL)
		return i.handlePubRel(ctx, p)

	case *packets.Pubcomp:
		metric.RecordMQTTReceivedPacket(packets.PUBCOMP)
		return i.handlePubComp(ctx, p)

	case *packets.Pingreq:
		metric.RecordMQTTReceivedPacket(packets.PINGREQ)
		return i.handlePing(ctx, p)

	case *packets.Disconnect:
		metric.RecordMQTTReceivedPacket(packets.DISCONNECT)
		return i.handleDisconnect(ctx, p)

	case *packets.Auth:
		metric.RecordMQTTReceivedPacket(packets.AUTH)
		return i.handleAuth(ctx, p)

	default:

		metric.RecordMQTTReceivedPacket(99)
		logger.Logger.Error().Str("client", client.metaString()).Msg("handle packet error")
	}
	return nil
}

// failProtocol 在协议错误时发送 DISCONNECT 并关闭连接。
func (i *InnerHandler) failProtocol(reason string) error {
	if i == nil || i.client == nil {
		return ErrProtocolError
	}
	if i.client.canSendDisconnect() {
		_ = i.client.write(&clientcap.WritePacket{Packet: disconnectForProtocolError(reason)})
	}
	_ = i.client.close()
	return ErrProtocolError
}

// failProtocolWithCode 与 failProtocol 类似，但允许调用方指定具体 ReasonCode（例如
// 0x81 Malformed Packet、0x82 Protocol Error、0x95 Packet Too Large 等）。
func (i *InnerHandler) failProtocolWithCode(code byte, reason string) error {
	if i == nil || i.client == nil {
		return ErrProtocolError
	}
	if i.client.canSendDisconnect() {
		_ = i.client.write(&clientcap.WritePacket{Packet: newServerDisconnect(code, reason)})
	}
	_ = i.client.close()
	return ErrProtocolError
}

// isClientToServerPacket 判断控制包类型是否允许由客户端上行发送。
func isClientToServerPacket(packetType packets.PacketType) bool {
	switch packetType {
	case packets.CONNECT,
		packets.PUBLISH,
		packets.PUBACK,
		packets.PUBREC,
		packets.PUBREL,
		packets.PUBCOMP,
		packets.SUBSCRIBE,
		packets.UNSUBSCRIBE,
		packets.PINGREQ,
		packets.DISCONNECT,
		packets.AUTH:
		return true
	default:
		return false
	}
}
