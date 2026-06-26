package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/acl"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	addr       string
	httpServer *gin.Engine
	server     *http.Server
	listener   net.Listener
	listenerMu sync.RWMutex

	tlsConfig config.TLS
	logConfig config.Log

	component *Component
}

type Component struct {
	// Component is kept for backward compatibility.
	ACLManager       *acl.Manager
	ACLAdminUsername string
	ACLAdminPassword string
	ClusterHealth    ClusterHealthProvider
	ClusterOverview  ClusterOverviewProvider
	Console          ConsoleProvider
	ConsoleEnabled   bool
	ConsoleUsername  string
	ConsolePassword  string
}

type ClusterHealthProvider interface {
	GetAllHealthStatus() map[uint64]*inner_cluster.ClusterHealthInfo
	GetHealthStatus(clusterID uint64) (*inner_cluster.ClusterHealthInfo, bool)
}

type ClusterOverviewProvider interface {
	GetClusterOverview(ctx context.Context) (*ClusterOverview, error)
}

func NewAPI(addr string, component *Component, tlsCfg ...config.TLS) *API {
	cfg := config.TLS{}
	if len(tlsCfg) > 0 {
		cfg = tlsCfg[0]
	}
	return NewAPIWithConfig(addr, component, cfg, config.Log{})
}

func NewAPIWithConfig(addr string, component *Component, tlsCfg config.TLS, logCfg config.Log) *API {
	api := &API{
		addr:      addr,
		component: component,
		tlsConfig: tlsCfg,
		logConfig: logCfg,
	}
	return api
}

func (a *API) Start(ctx context.Context) error {
	applyGinConfig(a.logConfig)
	r := gin.New()
	a.httpServer = r
	a.route()

	server := &http.Server{
		Addr:    a.addr,
		Handler: a.httpServer,
	}
	if a.tlsConfig.Enabled {
		tlsCfg, err := buildAPIServerTLSConfig(a.tlsConfig)
		if err != nil {
			return err
		}
		server.TLSConfig = tlsCfg
	}
	a.server = server

	listener, err := net.Listen("tcp", a.addr)
	if err != nil {
		return err
	}
	a.setListener(listener)

	errCh := make(chan error, 1)
	go func() {
		if a.tlsConfig.Enabled {
			errCh <- a.server.ServeTLS(listener, "", "")
			return
		}
		errCh <- a.server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		return a.Close()
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		logger.Logger.Error().Err(err).Msg("api server stopped unexpectedly")
		return err
	}
}

func applyGinConfig(cfg config.Log) {
	mode := strings.ToLower(strings.TrimSpace(cfg.GinMode))
	switch mode {
	case gin.DebugMode, gin.ReleaseMode, gin.TestMode:
	case "":
		mode = gin.ReleaseMode
	default:
		mode = gin.ReleaseMode
	}
	gin.SetMode(mode)

	if cfg.GinConsoleOutput {
		gin.DefaultWriter = os.Stderr
		gin.DefaultErrorWriter = os.Stderr
		return
	}
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
}

func buildAPIServerTLSConfig(cfg config.TLS) (*tls.Config, error) {
	certFile := strings.TrimSpace(cfg.CertFile)
	keyFile := strings.TrimSpace(cfg.KeyFile)
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("server.tls.cert_file and server.tls.key_file are required when server.tls.enabled=true")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server tls cert/key failed: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.MTLSAuthMode))
	switch mode {
	case "", "off":
		tlsCfg.ClientAuth = tls.NoClientCert
	case "optional", "required":
		caFile := strings.TrimSpace(cfg.CAFile)
		if caFile == "" {
			return nil, fmt.Errorf("server.tls.ca_file is required when server.tls.mtls_auth_mode=%s", mode)
		}
		pem, readErr := os.ReadFile(caFile)
		if readErr != nil {
			return nil, fmt.Errorf("read server tls ca_file failed: %w", readErr)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(pem); !ok {
			return nil, fmt.Errorf("parse server tls ca_file failed")
		}
		tlsCfg.ClientCAs = pool
		if mode == "required" {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
		}
	default:
		return nil, fmt.Errorf("invalid server.tls.mtls_auth_mode: %s", mode)
	}

	return tlsCfg, nil
}

func (a *API) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if a.server == nil {
		return nil
	}
	err := a.server.Shutdown(ctx)
	a.setListener(nil)
	return err
}

func (a *API) route() {
	a.httpServer.GET("/health", func(c *gin.Context) {
		a.writeHealthProbe(c, "health")
	})
	a.httpServer.GET("/health/liveness", func(c *gin.Context) {
		a.writeHealthProbe(c, "liveness")
	})
	a.httpServer.GET("/health/readiness", func(c *gin.Context) {
		a.writeHealthProbe(c, "readiness")
	})
	a.httpServer.GET("/health/startup", func(c *gin.Context) {
		a.writeHealthProbe(c, "startup")
	})

	// pprof endpoint
	a.httpServer.GET("/debug/pprof/*filepath", gin.WrapH(http.DefaultServeMux))

	// metrics endpoint
	a.httpServer.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := a.httpServer.Group("/api/v1")
	registerACLRoutes(v1, a.component)
	registerClusterRoutes(v1, a.component)
	registerConsoleRoutes(a, v1)
}

func (a *API) Name() string {
	return "api"
}

func (a *API) ListenerAddr() string {
	a.listenerMu.RLock()
	defer a.listenerMu.RUnlock()
	if a.listener == nil {
		return ""
	}
	return a.listener.Addr().String()
}

func (a *API) setListener(listener net.Listener) {
	a.listenerMu.Lock()
	a.listener = listener
	a.listenerMu.Unlock()
}

func (a *API) writeHealthProbe(c *gin.Context, probe string) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"probe":  probe,
	})
}
