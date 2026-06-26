package clientcap

import (
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ============================================================================
// 基础能力接口
// ============================================================================

// Identifier 标识符接口，提供获取ID的能力
type Identifier interface {
	GetID() string
}

// PacketIDGenerator 包ID生成器接口，用于生成MQTT包ID
type PacketIDGenerator interface {
	NextPacketID() uint16
}

// ============================================================================
// 写入器接口
// ============================================================================

// WritePacket 写入包结构体
type WritePacket struct {
	Packet *packets.ControlPacket
	// FullTopic is the full topic of the message.
	// If the message has a shared topic.
	// The FullTopic is the shared topic.
	// Like $share/shareGroup/topic
	FullTopic string

	SubscribeTopic string
}

// MessageWriter 消息写入器接口，提供写入和重试写入能力
type MessageWriter interface {
	Write(packet *WritePacket) error
	RetryWrite(packet *WritePacket) error
}

// PacketWriter 包写入器接口，组合了写入、ID生成和标识能力
type PacketWriter interface {
	MessageWriter
	PacketIDGenerator
	Identifier
	Close() error
}

// ============================================================================
// 发布者接口
// ============================================================================

// Publisher 发布者接口，提供发布消息和发布释放的能力
type Publisher interface {
	Publish(message *brokerpublish.Message) error
	PubRel(message *brokerpublish.Message) error
}

// ============================================================================
// 读取器接口
// ============================================================================

// MessageReader 消息读取器接口，提供获取未完成消息的能力
type MessageReader interface {
	GetUnFinishedMessage() []*brokerpublish.Message
}

// ============================================================================
// 发布响应处理器接口
// ============================================================================

// PublishResponseHandler 发布响应处理器接口，处理发布确认、接收和完成
type PublishResponseHandler interface {
	HandlePublishAck(pubAck *packets.Puback) error
	HandlePublishRec(pubRec *packets.Pubrec) error
	HandlePublishComp(pubComp *packets.Pubcomp) error
}

// ============================================================================
// 客户端主接口
// ============================================================================

// Client 客户端接口，组合了所有客户端能力
type Client interface {
	Publisher
	MessageReader
	PacketWriter
	PublishResponseHandler
	Close() error
}

// PublishDoneClient 发布路由处理所需的客户端接口。
// 这是一个最小接口，只包含发布路由需要读取的客户端能力，方便 mock 和测试。
type PublishDoneClient interface {
	Identifier
}
