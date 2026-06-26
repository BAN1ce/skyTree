package client

import packets "github.com/BAN1ce/skyTree/pkg/mqtt5"

// applyProblemInfoPreference 根据 CONNECT 中的 RequestProblemInformation 标志，
// 在 RequestProblemInformation=0 时移除 PUBACK/PUBREC/PUBREL/PUBCOMP/SUBACK/UNSUBACK/AUTH
// 上的 ReasonString 与 UserProperty。
//
// 按 MQTT 5.0 §3.1.2.11.7：当 RequestProblemInformation=0 时
//   - CONNACK / DISCONNECT / PUBLISH ：MAY 发送 ReasonString/UserProperty（服务端可选保留）。
//   - 其它报文：MUST NOT 发送。
//
// 因此本实现仅对"其它报文"做属性抹除。
func (c *Client) applyProblemInfoPreference(cp *packets.ControlPacket) {
	if c == nil || c.requestProblemInfo || cp == nil {
		return
	}
	switch p := cp.Content.(type) {
	case *packets.Puback:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Pubrec:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Pubrel:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Pubcomp:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Suback:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Unsuback:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	case *packets.Auth:
		if p.Properties != nil {
			p.Properties.ReasonString = ""
			p.Properties.User = nil
		}
	}
}
