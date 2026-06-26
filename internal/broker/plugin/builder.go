package plugin

import "github.com/BAN1ce/skyTree/config"

// Builder 插件构建器
// 提供便捷的插件注册方法
type Builder struct {
	plugins *Plugins
}

// NewBuilder 创建插件构建器
func NewBuilder() *Builder {
	return &Builder{
		plugins: &Plugins{
			PacketPlugin: PacketPlugin{},
			ClientPlugin: ClientPlugin{},
		},
	}
}

// AddMetric 添加基础监控插件
func (b *Builder) AddMetric() *Builder {
	metricPlugin := NewMetricPlugin()

	b.plugins.OnReceivedConnect = append(b.plugins.OnReceivedConnect, metricPlugin.OnReceivedConnect)
	b.plugins.OnSendConnAck = append(b.plugins.OnSendConnAck, metricPlugin.OnSendConnAck)
	b.plugins.OnSubscribe = append(b.plugins.OnSubscribe, metricPlugin.OnReceivedSubscribe)
	b.plugins.OnSendSubAck = append(b.plugins.OnSendSubAck, metricPlugin.OnSendSubAck)
	b.plugins.OnUnsubscribe = append(b.plugins.OnUnsubscribe, metricPlugin.OnReceivedUnsubscribe)
	b.plugins.OnSendUnsubAck = append(b.plugins.OnSendUnsubAck, metricPlugin.OnSendUnsubAck)
	b.plugins.OnReceivedPublish = append(b.plugins.OnReceivedPublish, metricPlugin.OnReceivedPublish)
	b.plugins.OnSendPublish = append(b.plugins.OnSendPublish, metricPlugin.OnSendPublish)
	b.plugins.OnReceivedPubAck = append(b.plugins.OnReceivedPubAck, metricPlugin.OnReceivedPubAck)
	b.plugins.OnSendPubAck = append(b.plugins.OnSendPubAck, metricPlugin.OnSendPubAck)
	b.plugins.OnReceivedPubRel = append(b.plugins.OnReceivedPubRel, metricPlugin.OnReceivedPubRel)
	b.plugins.OnSendPubRel = append(b.plugins.OnSendPubRel, metricPlugin.OnSendPubRel)
	b.plugins.OnReceivedPubRec = append(b.plugins.OnReceivedPubRec, metricPlugin.OnReceivedPubRec)
	b.plugins.OnSendPubRec = append(b.plugins.OnSendPubRec, metricPlugin.OnSendPubRec)
	b.plugins.OnReceivedPubComp = append(b.plugins.OnReceivedPubComp, metricPlugin.OnReceivedPubComp)
	b.plugins.OnSendPubComp = append(b.plugins.OnSendPubComp, metricPlugin.OnSendPubComp)
	b.plugins.OnReceivedPingReq = append(b.plugins.OnReceivedPingReq, metricPlugin.OnReceivedPingReq)
	b.plugins.OnSendPingResp = append(b.plugins.OnSendPingResp, metricPlugin.OnSendPingResp)
	b.plugins.OnReceivedDisconnect = append(b.plugins.OnReceivedDisconnect, metricPlugin.OnReceivedDisconnect)

	return b
}

// AddAuth 添加认证插件
func (b *Builder) AddAuth() *Builder {
	authPlugin := NewAuthPlugin()

	b.plugins.OnReceivedConnect = append(b.plugins.OnReceivedConnect, authPlugin.OnReceivedConnect)
	b.plugins.OnSubscribe = append(b.plugins.OnSubscribe, authPlugin.OnSubscribe)
	b.plugins.OnReceivedPublish = append(b.plugins.OnReceivedPublish, authPlugin.OnReceivedPublish)

	return b
}

// AddAuthPacket 添加AUTH报文处理插件
func (b *Builder) AddAuthPacket(authCfg config.AuthConfig) *Builder {
	authPacketPlugin := NewAuthPacketPlugin(authCfg)
	if authPacketPlugin != nil {
		b.plugins.OnReceivedAuth = append(b.plugins.OnReceivedAuth, authPacketPlugin.OnReceivedAuth)
	}

	return b
}

// AddErrorHandler 添加错误处理插件
func (b *Builder) AddErrorHandler() *Builder {
	errorPlugin := DefaultErrorHandlerPlugin()

	b.plugins.OnClientError = append(b.plugins.OnClientError, errorPlugin.Build().OnClientError...)

	return b
}

// Build 构建最终的插件配置
func (b *Builder) Build() *Plugins {
	return b.plugins
}

// 使用示例：
/*
func setupPlugins() *plugin.Plugins {
	return plugin.NewBuilder().
		AddMetric().  // 添加基础监控
		AddAuth().    // 添加认证
		Build()
}
*/
