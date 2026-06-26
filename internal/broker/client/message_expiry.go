package client

import (
	"time"
	"unicode/utf8"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// outboundPayloadFormatInvalid 检查投递前 PUBLISH 的 PFI 与 payload 是否一致。
// MQTT5 §3.3.2.3.2：PFI=1 的报文 payload 必须是合法 UTF-8。
// 入站校验过一次，但 retained / 跨节点投递会经过持久化与重建，投递前仍需再校验一次。
func outboundPayloadFormatInvalid(p *packets.Publish) bool {
	if p == nil || p.Properties == nil || p.Properties.PayloadFormat == nil {
		return false
	}
	return *p.Properties.PayloadFormat == 1 && !utf8.Valid(p.Payload)
}

// applyMessageExpiryForDelivery 在投递前根据 Message.ExpiredTime（绝对纳秒时间戳）
// 重新计算 PUBLISH 的 MessageExpiry 属性（剩余秒数）。
//
// 设计原则：
//   - ExpiredTime 是消息进入 broker 时一次性换算好的"绝对过期时刻"（见 SavePublishMessage）。
//   - 投递时只读 ExpiredTime；不再回退到 CreatedTime + MessageExpiry，
//     避免上一次投递把 publish.Properties.MessageExpiry 减成残量后被再次衰减。
//
// 返回值：是否仍可投递（false 表示已过期，应丢弃且推进 cursor）。
func applyMessageExpiryForDelivery(publish *packets.Publish, msg *brokerpublish.Message, now time.Time) bool {
	if publish == nil || msg == nil {
		return true
	}
	deadline := msg.ExpiredTime
	if deadline == 0 {
		// 没有过期时间约束。清掉 publish 上可能残留的 MessageExpiry，避免误导客户端。
		if publish.Properties != nil {
			publish.Properties.MessageExpiry = nil
		}
		return true
	}
	remainingNano := deadline - now.UnixNano()
	if remainingNano <= 0 {
		return false
	}
	if publish.Properties == nil {
		publish.Properties = &packets.PublishProperties{}
	}
	remaining := uint32((remainingNano + int64(time.Second) - 1) / int64(time.Second))
	if remaining == 0 {
		return false
	}
	publish.Properties.MessageExpiry = &remaining
	return true
}
