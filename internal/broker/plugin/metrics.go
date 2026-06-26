package plugin

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// MetricPlugin 基础监控插件
// 负责收集所有MQTT包的统计信息
type MetricPlugin struct {
	enableDetailedMetrics bool
}

// NewMetricPlugin 创建基础监控插件
func NewMetricPlugin() *MetricPlugin {
	return &MetricPlugin{
		enableDetailedMetrics: true,
	}
}

// 连接相关监控
func (m *MetricPlugin) OnReceivedConnect(ctx context.Context, clientID string, connect *packets.Connect) error {
	metric.RecordMQTTReceivedPacket(packets.CONNECT)
	return nil
}

func (m *MetricPlugin) OnSendConnAck(ctx context.Context, clientID string, connAck *packets.ConnAck) error {
	metric.RecordMQTTSentPacket(packets.CONNACK)
	return nil
}

func (m *MetricPlugin) OnReceivedDisconnect(ctx context.Context, clientID string, disconnect *packets.Disconnect) error {
	metric.RecordMQTTReceivedPacket(packets.DISCONNECT)
	return nil
}

// 订阅相关监控
func (m *MetricPlugin) OnReceivedSubscribe(ctx context.Context, clientID string, subscribe *packets.Subscribe) error {
	metric.RecordMQTTReceivedPacket(packets.SUBSCRIBE)
	return nil
}

func (m *MetricPlugin) OnSendSubAck(ctx context.Context, clientID string, subAck *packets.Suback) error {
	metric.RecordMQTTSentPacket(packets.SUBACK)
	return nil
}

func (m *MetricPlugin) OnReceivedUnsubscribe(ctx context.Context, clientID string, unsubscribe *packets.Unsubscribe) error {
	metric.RecordMQTTReceivedPacket(packets.UNSUBSCRIBE)
	return nil
}

func (m *MetricPlugin) OnSendUnsubAck(ctx context.Context, clientID string, unsubAck *packets.Unsuback) error {
	metric.RecordMQTTSentPacket(packets.UNSUBACK)
	return nil
}

// 发布相关监控
func (m *MetricPlugin) OnReceivedPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	metric.RecordMQTTReceivedPacket(packets.PUBLISH)
	metric.RecordMQTTPublishPacket(publish.QoS, "in")
	return nil
}

func (m *MetricPlugin) OnSendPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	metric.RecordMQTTSentPacket(packets.PUBLISH)
	metric.RecordMQTTPublishPacket(publish.QoS, "out")
	return nil
}

// QoS确认相关监控
func (m *MetricPlugin) OnReceivedPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	metric.RecordMQTTReceivedPacket(packets.PUBACK)
	return nil
}

func (m *MetricPlugin) OnSendPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	metric.RecordMQTTSentPacket(packets.PUBACK)
	return nil
}

func (m *MetricPlugin) OnReceivedPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	metric.RecordMQTTReceivedPacket(packets.PUBREL)
	return nil
}

func (m *MetricPlugin) OnSendPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	metric.RecordMQTTSentPacket(packets.PUBREL)
	return nil
}

func (m *MetricPlugin) OnReceivedPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	metric.RecordMQTTReceivedPacket(packets.PUBREC)
	return nil
}

func (m *MetricPlugin) OnSendPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	metric.RecordMQTTSentPacket(packets.PUBREC)
	return nil
}

func (m *MetricPlugin) OnReceivedPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	metric.RecordMQTTReceivedPacket(packets.PUBCOMP)
	return nil
}

func (m *MetricPlugin) OnSendPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	metric.RecordMQTTSentPacket(packets.PUBCOMP)
	return nil
}

// 心跳相关监控
func (m *MetricPlugin) OnReceivedPingReq(ctx context.Context, clientID string, pingReq *packets.Pingreq) error {
	metric.RecordMQTTReceivedPacket(packets.PINGREQ)
	return nil
}

func (m *MetricPlugin) OnSendPingResp(ctx context.Context, clientID string, pingResp *packets.Pingresp) error {
	metric.RecordMQTTSentPacket(packets.PINGRESP)
	return nil
}

// AdvancedMetricPlugin 高级监控插件
// 支持性能监控、业务指标等
type AdvancedMetricPlugin struct {
	enablePerformanceMetrics bool
	enableBusinessMetrics    bool
}

// NewAdvancedMetricPlugin 创建高级监控插件
func NewAdvancedMetricPlugin() *AdvancedMetricPlugin {
	return &AdvancedMetricPlugin{
		enablePerformanceMetrics: true,
		enableBusinessMetrics:    true,
	}
}

// 连接监控 - 包含性能指标
func (m *AdvancedMetricPlugin) OnReceivedConnect(ctx context.Context, clientID string, connect *packets.Connect) error {
	startTime := time.Now()
	defer func() {
		if m.enablePerformanceMetrics {
			duration := time.Since(startTime)
			logger.Logger.Debug().
				Str("client", clientID).
				Dur("duration", duration).
				Msg("connect processing time")
		}
	}()

	// 基础metric
	metric.RecordMQTTReceivedPacket(packets.CONNECT)

	// 业务指标
	if m.enableBusinessMetrics {
		logger.Logger.Debug().
			Str("client", clientID).
			Str("protocol", connect.ProtocolName).
			Int("version", int(connect.ProtocolVersion)).
			Msg("client connection metrics")
	}

	return nil
}

// 发布监控 - 包含消息大小、QoS分布等
func (m *AdvancedMetricPlugin) OnReceivedPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	startTime := time.Now()
	defer func() {
		if m.enablePerformanceMetrics {
			duration := time.Since(startTime)
			logger.Logger.Debug().
				Str("client", clientID).
				Str("topic", publish.Topic).
				Dur("duration", duration).
				Msg("publish processing time")
		}
	}()

	// 基础metric
	metric.RecordMQTTReceivedPacket(packets.PUBLISH)
	metric.RecordMQTTPublishPacket(publish.QoS, "in")

	// 高级指标
	if m.enableBusinessMetrics {
		messageSize := len(publish.Payload)
		logger.Logger.Debug().
			Str("client", clientID).
			Str("topic", publish.Topic).
			Int("size", messageSize).
			Int("qos", int(publish.QoS)).
			Msg("publish message metrics")
	}

	return nil
}

// 发送发布监控
func (m *AdvancedMetricPlugin) OnSendPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	// 基础metric
	metric.RecordMQTTSentPacket(packets.PUBLISH)
	metric.RecordMQTTPublishPacket(publish.QoS, "out")

	// 高级指标
	if m.enableBusinessMetrics {
		messageSize := len(publish.Payload)
		logger.Logger.Debug().
			Str("client", clientID).
			Str("topic", publish.Topic).
			Int("size", messageSize).
			Int("qos", int(publish.QoS)).
			Msg("send publish message metrics")
	}

	return nil
}

// 订阅监控 - 包含订阅模式分析
func (m *AdvancedMetricPlugin) OnReceivedSubscribe(ctx context.Context, clientID string, subscribe *packets.Subscribe) error {
	// 基础metric
	metric.RecordMQTTReceivedPacket(packets.SUBSCRIBE)

	// 高级指标
	if m.enableBusinessMetrics {
		for _, topic := range subscribe.Subscriptions {
			logger.Logger.Debug().
				Str("client", clientID).
				Str("topic", topic.Topic).
				Msg("subscription metrics")
		}
	}

	return nil
}

// 其他方法保持基础实现
func (m *AdvancedMetricPlugin) OnSendConnAck(ctx context.Context, clientID string, connAck *packets.ConnAck) error {
	metric.RecordMQTTSentPacket(packets.CONNACK)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendSubAck(ctx context.Context, clientID string, subAck *packets.Suback) error {
	metric.RecordMQTTSentPacket(packets.SUBACK)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedUnsubscribe(ctx context.Context, clientID string, unsubscribe *packets.Unsubscribe) error {
	metric.RecordMQTTReceivedPacket(packets.UNSUBSCRIBE)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendUnsubAck(ctx context.Context, clientID string, unsubAck *packets.Unsuback) error {
	metric.RecordMQTTSentPacket(packets.UNSUBACK)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	metric.RecordMQTTReceivedPacket(packets.PUBACK)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	metric.RecordMQTTSentPacket(packets.PUBACK)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	metric.RecordMQTTReceivedPacket(packets.PUBREL)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	metric.RecordMQTTSentPacket(packets.PUBREL)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	metric.RecordMQTTReceivedPacket(packets.PUBREC)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	metric.RecordMQTTSentPacket(packets.PUBREC)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	metric.RecordMQTTReceivedPacket(packets.PUBCOMP)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	metric.RecordMQTTSentPacket(packets.PUBCOMP)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedPingReq(ctx context.Context, clientID string, pingReq *packets.Pingreq) error {
	metric.RecordMQTTReceivedPacket(packets.PINGREQ)
	return nil
}

func (m *AdvancedMetricPlugin) OnSendPingResp(ctx context.Context, clientID string, pingResp *packets.Pingresp) error {
	metric.RecordMQTTSentPacket(packets.PINGRESP)
	return nil
}

func (m *AdvancedMetricPlugin) OnReceivedDisconnect(ctx context.Context, clientID string, disconnect *packets.Disconnect) error {
	metric.RecordMQTTReceivedPacket(packets.DISCONNECT)
	return nil
}
