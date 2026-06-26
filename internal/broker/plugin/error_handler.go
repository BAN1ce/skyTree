package plugin

import (
	"context"

	"github.com/BAN1ce/skyTree/logger"
)

// ErrorHandlerPlugin 错误处理插件
// 用于处理客户端错误，如限流、认证失败等
type ErrorHandlerPlugin struct {
	// 是否启用错误日志记录
	EnableLogging bool
	// 是否启用错误统计
	EnableMetrics bool
}

// NewErrorHandlerPlugin 创建错误处理插件
func NewErrorHandlerPlugin() *ErrorHandlerPlugin {
	return &ErrorHandlerPlugin{
		EnableLogging: true,
		EnableMetrics: true,
	}
}

// WithLogging 设置是否启用日志记录
func (p *ErrorHandlerPlugin) WithLogging(enable bool) *ErrorHandlerPlugin {
	p.EnableLogging = enable
	return p
}

// WithMetrics 设置是否启用统计
func (p *ErrorHandlerPlugin) WithMetrics(enable bool) *ErrorHandlerPlugin {
	p.EnableMetrics = enable
	return p
}

// Build 构建插件
func (p *ErrorHandlerPlugin) Build() *Plugins {
	return &Plugins{
		ClientPlugin: ClientPlugin{
			OnClientError: []OnClientError{
				p.handleClientError,
			},
		},
	}
}

// handleClientError 处理客户端错误
func (p *ErrorHandlerPlugin) handleClientError(ctx context.Context, clientID string, err error) error {
	if p.EnableLogging {
		logger.Logger.Error().
			Str("client", clientID).
			Err(err).
			Msg("client error occurred")
	}

	if p.EnableMetrics {
		// TODO: 这里可以添加错误统计metric
		// 例如：metric.IncrementErrorCounter(clientID, err.Error())
	}

	// 可以根据错误类型进行不同的处理
	switch {
	case isRateLimitError(err):
		logger.Logger.Warn().
			Str("client", clientID).
			Msg("client rate limit exceeded, connection will be closed")
	case isAuthError(err):
		logger.Logger.Warn().
			Str("client", clientID).
			Msg("client authentication failed, connection will be closed")
	default:
		logger.Logger.Error().
			Str("client", clientID).
			Err(err).
			Msg("unknown client error, connection will be closed")
	}

	// 返回错误，让上层处理（通常是关闭客户端连接）
	return err
}

// isRateLimitError 判断是否为限流错误
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// 检查错误信息是否包含限流相关关键词
	errMsg := err.Error()
	return containsAny(errMsg, []string{"rate limit", "rate_limit", "限流", "too many"})
}

// isAuthError 判断是否为认证错误
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	// 检查错误信息是否包含认证相关关键词
	errMsg := err.Error()
	return containsAny(errMsg, []string{"auth", "authentication", "unauthorized", "认证", "权限"})
}

// containsAny 检查字符串是否包含任意一个关键词
func containsAny(s string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(keyword) > 0 && len(s) >= len(keyword) {
			// 简单的包含检查，实际项目中可以使用更复杂的匹配逻辑
			for i := 0; i <= len(s)-len(keyword); i++ {
				if s[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	return false
}

// DefaultErrorHandlerPlugin 创建默认的错误处理插件
func DefaultErrorHandlerPlugin() *ErrorHandlerPlugin {
	return NewErrorHandlerPlugin()
}
