package plugin

import (
	"context"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/auth"
	auth_grpc "github.com/BAN1ce/skyTree/internal/broker/auth"
	auth_http "github.com/BAN1ce/skyTree/internal/broker/auth"
	"github.com/BAN1ce/skyTree/logger"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// AuthPacketPlugin AUTH报文处理插件
type AuthPacketPlugin struct {
	provider auth.AuthProvider
}

// NewAuthPacketPlugin 创建AUTH报文处理插件
func NewAuthPacketPlugin(authCfg config.AuthConfig) *AuthPacketPlugin {
	// 如果未启用，返回nil
	if !authCfg.Enabled {
		return nil
	}

	plugin := &AuthPacketPlugin{}

	// 根据配置选择HTTP或gRPC AuthProvider
	// 优先级：HTTP > gRPC
	if authCfg.HTTPEndpoint != "" {
		provider := auth_http.NewHTTPProvider(authCfg.HTTPEndpoint, authCfg.Timeout)
		plugin.provider = provider
		logger.Logger.Info().Str("endpoint", authCfg.HTTPEndpoint).
			Msg("AUTH plugin using HTTP provider")
	} else if authCfg.GRPCEndpoint != "" {
		provider, err := auth_grpc.NewGRPCProvider(authCfg.GRPCEndpoint, authCfg.Timeout)
		if err != nil {
			logger.Logger.Error().Err(err).Str("endpoint", authCfg.GRPCEndpoint).
				Msg("failed to create gRPC auth provider, AUTH will not work")
			return nil
		}
		plugin.provider = provider
		logger.Logger.Info().Str("endpoint", authCfg.GRPCEndpoint).
			Msg("AUTH plugin using gRPC provider")
	} else {
		// 没有配置端点，返回nil（不启用插件）
		logger.Logger.Debug().Msg("AUTH plugin disabled: no endpoint configured")
		return nil
	}

	return plugin
}

// OnReceivedAuth 处理接收到的AUTH报文
func (p *AuthPacketPlugin) OnReceivedAuth(ctx context.Context, clientID string, authPacket *packets.Auth) (*packets.Auth, error) {
	if p == nil || p.provider == nil {
		// 如果没有provider，返回原报文（允许其他插件处理）
		return authPacket, nil
	}

	// 调用AuthProvider进行认证
	result, err := p.provider.Authenticate(ctx, clientID, authPacket)
	if err != nil {
		logger.Logger.Error().Err(err).Str("client", clientID).
			Msg("AUTH provider authentication failed")
		// 返回需要重新认证
		return &packets.Auth{
			ReasonCode: packets.AuthReauthenticate,
		}, nil
	}

	return result, nil
}

// Close 关闭插件资源
func (p *AuthPacketPlugin) Close() error {
	if p == nil {
		return nil
	}

	// 如果provider实现了Close方法，调用它
	if closer, ok := p.provider.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}
