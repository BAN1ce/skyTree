package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	proto_auth "github.com/BAN1ce/skyTree/proto/proto_auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// GRPCProvider gRPC认证提供者
type GRPCProvider struct {
	endpoint string
	timeout  time.Duration
	mu       sync.RWMutex
	conn     *grpc.ClientConn
	client   proto_auth.AuthServiceClient
}

// NewGRPCProvider 创建gRPC认证提供者
func NewGRPCProvider(endpoint string, timeoutSeconds int) (*GRPCProvider, error) {
	validTimeout := ValidateTimeout(timeoutSeconds)
	timeout := time.Duration(validTimeout) * time.Second

	provider := &GRPCProvider{
		endpoint: endpoint,
		timeout:  timeout,
	}

	// 尝试建立连接
	if err := provider.connect(context.Background()); err != nil {
		logger.Logger.Warn().Err(err).Str("endpoint", endpoint).
			Msg("failed to dial gRPC auth endpoint, will retry on request")
		// 不返回错误，允许延迟连接
	}

	return provider, nil
}

// connect 建立gRPC连接
func (p *GRPCProvider) connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果连接已存在且有效，直接返回
	if p.conn != nil {
		state := p.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return nil
		}
		// 连接状态异常，关闭旧连接
		_ = p.conn.Close()
		p.conn = nil
		p.client = nil
	}

	// 建立连接
	conn, err := grpc.NewClient(p.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to dial gRPC endpoint: %w", err)
	}

	p.conn = conn
	p.client = proto_auth.NewAuthServiceClient(conn)
	return nil
}

// getClient 获取gRPC客户端，如果连接不存在则尝试重连
func (p *GRPCProvider) getClient(ctx context.Context) (proto_auth.AuthServiceClient, error) {
	p.mu.RLock()
	if p.client != nil && p.conn != nil {
		state := p.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			client := p.client
			p.mu.RUnlock()
			return client, nil
		}
	}
	p.mu.RUnlock()

	// 需要重新连接
	if err := p.connect(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client, nil
}

// Authenticate 执行gRPC认证请求
func (p *GRPCProvider) Authenticate(ctx context.Context, clientID string, authPacket *packets.Auth) (*packets.Auth, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		metric.RecordAuthDuration("grpc", duration)
	}()

	// 创建带超时的context，最大10秒
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// 获取客户端
	client, err := p.getClient(reqCtx)
	if err != nil {
		metric.RecordAuthRequestFailed("grpc", "connection_error")
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("failed to get gRPC client")
		return nil, fmt.Errorf("failed to get gRPC client: %w", err)
	}

	// 构建请求
	req := &proto_auth.AuthRequest{
		ClientId:   clientID,
		ReasonCode: uint32(authPacket.ReasonCode),
	}

	if authPacket.Properties != nil {
		req.AuthMethod = authPacket.Properties.AuthMethod
		if len(authPacket.Properties.AuthData) > 0 {
			req.AuthData = authPacket.Properties.AuthData
		}
		if authPacket.Properties.ReasonString != "" {
			req.ReasonString = authPacket.Properties.ReasonString
		}
		if len(authPacket.Properties.User) > 0 {
			req.UserProperties = make([]*proto_auth.UserProperty, 0, len(authPacket.Properties.User))
			for _, u := range authPacket.Properties.User {
				req.UserProperties = append(req.UserProperties, &proto_auth.UserProperty{
					Key:   u.Key,
					Value: u.Value,
				})
			}
		}
	}

	// 发送请求
	resp, err := client.Authenticate(reqCtx, req)
	if err != nil {
		errorType := "rpc_error"
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			errorType = "timeout"
			logger.Logger.Warn().Err(err).Str("client", clientID).Dur("timeout", p.timeout).
				Msg("AUTH gRPC request timeout")
		} else {
			logger.Logger.Error().Err(err).Str("client", clientID).Msg("AUTH gRPC request failed")
		}
		metric.RecordAuthRequestFailed("grpc", errorType)
		return nil, fmt.Errorf("gRPC request failed: %w", err)
	}

	// 构建AUTH响应报文
	authResp := &packets.Auth{
		ReasonCode: byte(resp.ReasonCode),
	}

	if resp.AuthMethod != "" || len(resp.AuthData) > 0 || resp.ReasonString != "" || len(resp.UserProperties) > 0 {
		authResp.Properties = &packets.AuthProperties{
			AuthMethod:   resp.AuthMethod,
			AuthData:     resp.AuthData,
			ReasonString: resp.ReasonString,
		}

		if len(resp.UserProperties) > 0 {
			authResp.Properties.User = make([]packets.User, 0, len(resp.UserProperties))
			for _, up := range resp.UserProperties {
				authResp.Properties.User = append(authResp.Properties.User, packets.User{
					Key:   up.Key,
					Value: up.Value,
				})
			}
		}
	}

	// 记录成功metric
	if authResp.ReasonCode == packets.AuthSuccess {
		metric.RecordAuthRequest("grpc", "success")
	} else {
		metric.RecordAuthRequest("grpc", "failed")
	}

	return authResp, nil
}

// Close 关闭gRPC连接
func (p *GRPCProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		p.client = nil
		return err
	}
	return nil
}
