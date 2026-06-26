package disconnect

import (
	"strings"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// Option 用来组合 DISCONNECT 报文的可选属性。
type Option func(*packets.DisconnectProperties)

// WithServerReference 指定 Server Reference，用于 0x9C/0x9D 等重定向场景。
func WithServerReference(ref string) Option {
	return func(p *packets.DisconnectProperties) {
		if ref != "" {
			p.ServerReference = ref
		}
	}
}

// WithUserProperty 追加一个 User Property 键值对。
func WithUserProperty(key, value string) Option {
	return func(p *packets.DisconnectProperties) {
		p.User = append(p.User, packets.User{Key: key, Value: value})
	}
}

// NewServerDisconnect 是所有服务端主动 DISCONNECT 的统一构造入口。
//
// 不依赖 client 实例，由调用方（client.write 路径）按 RequestProblemInformation
// 决定是否在写入前抹除 ReasonString/UserProperty。
func NewServerDisconnect(code byte, reason string, opts ...Option) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	disc := &packets.Disconnect{ReasonCode: code}
	if reason != "" || len(opts) > 0 {
		props := &packets.DisconnectProperties{}
		if reason != "" {
			props.ReasonString = strings.TrimSpace(reason)
		}
		for _, opt := range opts {
			if opt != nil {
				opt(props)
			}
		}
		disc.Properties = props
	}
	cp.Content = disc
	return cp
}

// NewServerDisconnectMinimal 构造一个不带任何属性的 DISCONNECT，用于
// 因报文长度限制必须裸发的兜底场景。
func NewServerDisconnectMinimal(code byte) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.DISCONNECT)
	cp.Content = &packets.Disconnect{ReasonCode: code}
	return cp
}
