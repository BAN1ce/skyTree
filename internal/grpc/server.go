package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	broker_session "github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	subscription "github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"net"
)

var gracefulStopTimeout = 5 * time.Second

type Server struct {
	server        *grpc.Server
	addr          string
	listener      net.Listener
	publish       *eventbus.EventCenter[*delivery_event.Notify]
	clientManager *client.Manager
	subCenter     subscription.Center
	sessionCenter broker_session.Center
	sharedManager func() *shared_manager.SharedSubscriptionManager
	localNodeID   uint64
	tlsConfig     config.TLS
	allowInsecure bool
}

func NewServer(addr string, center *eventbus.EventCenter[*delivery_event.Notify], manager *client.Manager, subCenter subscription.Center, sessionCenter broker_session.Center, sharedManager func() *shared_manager.SharedSubscriptionManager, localNodeID uint64, tlsCfg config.TLS, allowInsecure bool) *Server {
	return &Server{
		addr:          addr,
		publish:       center,
		clientManager: manager,
		subCenter:     subCenter,
		sessionCenter: sessionCenter,
		sharedManager: sharedManager,
		localNodeID:   localNodeID,
		tlsConfig:     tlsCfg,
		allowInsecure: allowInsecure,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if logger.Logger != nil {
		logger.Logger.Info().Str("addr", s.addr).Msg("starting gRPC server")
	}

	var (
		clientDeliveryNotify = NewClientDeliveryNotifyGRPCServer(s.publish, func() sharedWakeHandler {
			if s.sharedManager == nil {
				return nil
			}
			return s.sharedManager()
		})
		err error
	)

	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	grpcOptions, err := s.serverOptions()
	if err != nil {
		_ = s.listener.Close()
		return err
	}
	s.server = grpc.NewServer(grpcOptions...)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	nodepb.RegisterClientDeliveryNotifyServer(s.server, clientDeliveryNotify)
	nodepb.RegisterClientCenterServer(s.server, NewServiceCloseClient(s.clientManager))
	grpc_health_v1.RegisterHealthServer(s.server, healthServer)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- s.server.Serve(s.listener)
	}()

	select {
	case <-ctx.Done():
		return s.Close()
	case serveErr := <-serveErrCh:
		if serveErr == nil || errors.Is(serveErr, net.ErrClosed) {
			return nil
		}
		return serveErr
	}
}

func (s *Server) Name() string {
	return "grpc"
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if s.server != nil {
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()
		timer := time.NewTimer(gracefulStopTimeout)
		defer timer.Stop()
		select {
		case <-stopped:
		case <-timer.C:
			s.server.Stop()
			<-stopped
		}
	}
	if s.listener == nil {
		return nil
	}
	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil

}

func (s *Server) serverOptions() ([]grpc.ServerOption, error) {
	options := []grpc.ServerOption{
		grpc.UnaryInterceptor(metricsUnaryServerInterceptor()),
	}
	if s.tlsConfig.Enabled {
		creds, err := s.buildTLSCredentials()
		if err != nil {
			return nil, err
		}
		return append(options, grpc.Creds(creds)), nil
	}

	if s.allowInsecure {
		if logger.Logger != nil {
			logger.Logger.Warn().Msg("cluster gRPC server is running without TLS (allow_insecure=true)")
		}
		return options, nil
	}

	return nil, fmt.Errorf("cluster gRPC server TLS is disabled: set cluster.grpc.tls.enabled=true or cluster.grpc.allow_insecure=true")
}

func (s *Server) buildTLSCredentials() (credentials.TransportCredentials, error) {
	certFile := strings.TrimSpace(s.tlsConfig.CertFile)
	keyFile := strings.TrimSpace(s.tlsConfig.KeyFile)
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("cluster.grpc.tls.cert_file and key_file are required when cluster.grpc.tls.enabled=true")
	}

	certPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load cluster grpc tls cert/key failed: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certPair},
	}

	mode := strings.ToLower(strings.TrimSpace(s.tlsConfig.MTLSAuthMode))
	switch mode {
	case "", "off":
		tlsCfg.ClientAuth = tls.NoClientCert
	case "optional", "required":
		caFile := strings.TrimSpace(s.tlsConfig.CAFile)
		if caFile == "" {
			return nil, fmt.Errorf("cluster.grpc.tls.ca_file is required when mtls_auth_mode=%s", mode)
		}
		pemData, readErr := os.ReadFile(caFile)
		if readErr != nil {
			return nil, fmt.Errorf("read cluster grpc tls ca_file failed: %w", readErr)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("parse cluster grpc tls ca_file failed")
		}
		tlsCfg.ClientCAs = pool
		if mode == "required" {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
		}
	default:
		return nil, fmt.Errorf("invalid cluster.grpc.tls.mtls_auth_mode: %s", mode)
	}

	return credentials.NewTLS(tlsCfg), nil
}
