package session

import (
	"bytes"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	subscription "github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

// ============================================================================
// MQTT Packet 到 Proto 结构的转换函数
// ============================================================================

// WillMessageToProto 将 MQTT Will Message 转换为 Proto 结构
func WillMessageToProto(connectPacket *packets.Connect) *proto_session.WillMessage {
	if connectPacket == nil || !connectPacket.WillFlag {
		return nil
	}

	willMsg := &proto_session.WillMessage{
		Topic:   connectPacket.WillTopic,
		Payload: connectPacket.WillMessage,
		Qos:     uint32(connectPacket.WillQOS),
		Retain:  connectPacket.WillRetain,
	}

	// MQTT 5.0 Will Properties
	if connectPacket.WillProperties != nil {
		// ContentType
		if connectPacket.WillProperties.ContentType != "" {
			willMsg.ContentType = &connectPacket.WillProperties.ContentType
		}

		// ResponseTopic
		if connectPacket.WillProperties.ResponseTopic != "" {
			willMsg.ResponseTopic = &connectPacket.WillProperties.ResponseTopic
		}

		// CorrelationData
		if len(connectPacket.WillProperties.CorrelationData) > 0 {
			correlationData := make([]byte, len(connectPacket.WillProperties.CorrelationData))
			copy(correlationData, connectPacket.WillProperties.CorrelationData)
			willMsg.CorrelationData = correlationData
		}

		// MessageExpiry (直接赋值，无需字符串转换)
		if connectPacket.WillProperties.MessageExpiry != nil {
			expiry := uint32(*connectPacket.WillProperties.MessageExpiry)
			willMsg.MessageExpiry = &expiry
		}

		if connectPacket.WillProperties.PayloadFormat != nil {
			payloadFormat := uint32(*connectPacket.WillProperties.PayloadFormat)
			willMsg.PayloadFormat = &payloadFormat
		}

		if connectPacket.WillProperties.WillDelayInterval != nil {
			willDelayInterval := uint32(*connectPacket.WillProperties.WillDelayInterval)
			willMsg.WillDelayInterval = &willDelayInterval
		}

		// User Properties (支持重复键)
		if len(connectPacket.WillProperties.User) > 0 {
			willMsg.UserProperties = make([]*proto_session.UserProperty, 0, len(connectPacket.WillProperties.User))
			for _, userProp := range connectPacket.WillProperties.User {
				willMsg.UserProperties = append(willMsg.UserProperties, &proto_session.UserProperty{
					Key:   userProp.Key,
					Value: userProp.Value,
				})
			}
		}
	}

	return willMsg
}

// ============================================================================
// Proto 结构到 MQTT Packet 的转换函数
// ============================================================================

// ProtoToWillProperties 将 Proto WillMessage 转换回 MQTT Will Properties
func ProtoToWillProperties(willMsg *proto_session.WillMessage) *packets.PublishProperties {
	if willMsg == nil {
		return nil
	}

	props := &packets.PublishProperties{
		User: []packets.User{},
	}

	// ContentType
	if willMsg.ContentType != nil && *willMsg.ContentType != "" {
		props.ContentType = *willMsg.ContentType
	}

	// ResponseTopic
	if willMsg.ResponseTopic != nil && *willMsg.ResponseTopic != "" {
		props.ResponseTopic = *willMsg.ResponseTopic
	}

	// CorrelationData
	if len(willMsg.CorrelationData) > 0 {
		correlationData := make([]byte, len(willMsg.CorrelationData))
		copy(correlationData, willMsg.CorrelationData)
		props.CorrelationData = correlationData
	}

	// MessageExpiry (直接赋值)
	if willMsg.MessageExpiry != nil {
		expiry := uint32(*willMsg.MessageExpiry)
		props.MessageExpiry = &expiry
	}

	if willMsg.PayloadFormat != nil {
		payloadFormat := byte(*willMsg.PayloadFormat)
		props.PayloadFormat = &payloadFormat
	}

	// User Properties (支持重复键)
	if len(willMsg.UserProperties) > 0 {
		props.User = make([]packets.User, 0, len(willMsg.UserProperties))
		for _, userProp := range willMsg.UserProperties {
			props.User = append(props.User, packets.User{
				Key:   userProp.Key,
				Value: userProp.Value,
			})
		}
	}

	return props
}

// ============================================================================
// 未完成消息转换函数
// ============================================================================

// ConvertToProtoUnfinishedMessage 将 broker publish message 转换为 proto UnfinishedMessage
// msg: 要转换的消息
// isOutgoing: true 表示 broker 发送给客户端的消息，false 表示客户端发送给 broker 的消息
func ConvertToProtoUnfinishedMessage(msg *brokerpublish.Message, isOutgoing bool) *proto_session.UnfinishedMessage {
	if msg == nil {
		return nil
	}

	publish := msg.GetPublish()
	if publish == nil {
		return nil
	}

	unfinished := &proto_session.UnfinishedMessage{
		MessageID:  msg.MessageID.String(),
		PacketID:   uint32(publish.PacketID),
		Qos:        uint32(publish.QoS),
		IsOutgoing: isOutgoing,
	}

	// 设置订阅主题（仅对 outgoing 消息有效）
	if isOutgoing && msg.SubscribeTopic != "" {
		unfinished.SubscribeTopic = &msg.SubscribeTopic
	}

	// 设置首次发布时间
	if msg.RetryInfo != nil {
		unfinished.FirstPubTime = msg.RetryInfo.FirstPubTime.UnixMicro()
	} else {
		unfinished.FirstPubTime = time.Now().UnixMicro()
	}

	// 根据 QoS 和状态确定消息状态
	if isOutgoing {
		switch publish.QoS {
		case subscription.QoS1:
			return nil
		case subscription.QoS2:
			if msg.PubReceived {
				unfinished.State = proto_session.UnfinishedMessage_WAITING_PUBCOMP
			} else {
				unfinished.State = proto_session.UnfinishedMessage_WAITING_PUBREC
			}
		default:
			return nil
		}
	} else {
		if publish.QoS != subscription.QoS2 {
			return nil
		}
		unfinished.State = proto_session.UnfinishedMessage_WAITING_PUBREL
		unfinished.AckReasonCode = uint32(msg.AckReasonCode)
		if cp := msg.GetControlPacket(); cp != nil {
			var buf bytes.Buffer
			if _, err := wire.Write(&buf, cp, wire.EncodeOptions{}); err == nil {
				unfinished.PublishPacket = buf.Bytes()
			}
		}
	}

	return unfinished
}

// ConvertToProtoUnfinishedMessages 批量转换未完成消息
func ConvertToProtoUnfinishedMessages(messages []*brokerpublish.Message, isOutgoing bool) []*proto_session.UnfinishedMessage {
	if len(messages) == 0 {
		return nil
	}

	result := make([]*proto_session.UnfinishedMessage, 0, len(messages))
	for _, msg := range messages {
		if protoMsg := ConvertToProtoUnfinishedMessage(msg, isOutgoing); protoMsg != nil {
			result = append(result, protoMsg)
		}
	}

	return result
}
