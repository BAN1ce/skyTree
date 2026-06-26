package auth

import (
	"context"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// AuthProvider 认证提供者接口
// 用于抽象不同的认证实现（HTTP、gRPC、插件等）
type AuthProvider interface {
	// Authenticate 执行认证请求
	// ctx: 上下文，用于超时控制
	// clientID: 客户端ID
	// auth: 接收到的AUTH报文
	// 返回: 响应AUTH报文和错误
	Authenticate(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error)
}

// ProviderType 提供者类型
type ProviderType string

const (
	ProviderTypeHTTP   ProviderType = "http"
	ProviderTypeGRPC   ProviderType = "grpc"
	ProviderTypePlugin ProviderType = "plugin"
)

// ValidateTimeout 验证并限制超时时间，最大值为10秒
func ValidateTimeout(timeoutSeconds int) int {
	const maxTimeout = 10
	if timeoutSeconds <= 0 {
		return 5 // 默认5秒
	}
	if timeoutSeconds > maxTimeout {
		return maxTimeout
	}
	return timeoutSeconds
}
