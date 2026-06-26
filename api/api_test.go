package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/acl"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

func TestApplyGinConfigDefaultsToSilentReleaseMode(t *testing.T) {
	originalMode := gin.Mode()
	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	t.Cleanup(func() {
		gin.SetMode(originalMode)
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
	})

	var out bytes.Buffer
	gin.DefaultWriter = &out
	gin.DefaultErrorWriter = &out

	applyGinConfig(config.Log{})
	_, _ = gin.DefaultWriter.Write([]byte("debug output"))
	_, _ = gin.DefaultErrorWriter.Write([]byte("error output"))

	if got := gin.Mode(); got != gin.ReleaseMode {
		t.Fatalf("expected gin mode %q, got %q", gin.ReleaseMode, got)
	}
	if out.Len() != 0 {
		t.Fatalf("expected gin default writers to be silent, got %q", out.String())
	}
}

func TestApplyGinConfigAllowsConsoleOutputWhenConfigured(t *testing.T) {
	originalMode := gin.Mode()
	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	t.Cleanup(func() {
		gin.SetMode(originalMode)
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
	})

	applyGinConfig(config.Log{
		GinMode:          gin.DebugMode,
		GinConsoleOutput: true,
	})

	if got := gin.Mode(); got != gin.DebugMode {
		t.Fatalf("expected gin mode %q, got %q", gin.DebugMode, got)
	}
	if gin.DefaultWriter == io.Discard {
		t.Fatalf("expected gin default writer to be enabled")
	}
	if gin.DefaultErrorWriter == io.Discard {
		t.Fatalf("expected gin default error writer to be enabled")
	}
}

func TestRouteHealthReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()
	a := NewAPI(":0", nil)
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected /health to return 200, got %d", rec.Code)
	}
}

func TestRouteHealthProbeEndpointsReturnOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", nil)
	a.httpServer = gin.New()
	a.route()

	for _, path := range []string{"/health/liveness", "/health/readiness", "/health/startup"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		a.httpServer.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d", path, rec.Code)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		if payload["status"] != "ok" {
			t.Fatalf("expected %s status ok, got %#v", path, payload["status"])
		}
	}
}

func TestACLRoutesDisabledWhenCredentialsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		ACLManager: &acl.Manager{},
	})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acl/ruleset", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected disabled ACL route to return 404, got %d", rec.Code)
	}
}

func TestACLRoutesRequireBasicAuthWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		ACLManager:       &acl.Manager{},
		ACLAdminUsername: "admin",
		ACLAdminPassword: "secret",
	})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acl/ruleset", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without auth header, got %d", rec.Code)
	}

	reqWithAuth := httptest.NewRequest(http.MethodGet, "/api/v1/acl/ruleset", nil)
	token := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	reqWithAuth.Header.Set("Authorization", "Basic "+token)
	recWithAuth := httptest.NewRecorder()
	a.httpServer.ServeHTTP(recWithAuth, reqWithAuth)
	if recWithAuth.Code != http.StatusInternalServerError {
		t.Fatalf("expected ACL handler execution after auth (500 for empty manager), got %d", recWithAuth.Code)
	}
}

func TestBuildAPIServerTLSConfigRequiresCAWhenMTLSRequired(t *testing.T) {
	_, err := buildAPIServerTLSConfig(config.TLS{
		Enabled:      true,
		CertFile:     "a",
		KeyFile:      "b",
		MTLSAuthMode: "required",
	})
	if err == nil {
		t.Fatalf("expected error when mtls required without ca_file")
	}
}

func TestAPIStartWithTLSEnabledServesHTTPSOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	certFile, keyFile := writeSelfSignedCert(t)
	a := NewAPI("127.0.0.1:0", nil, config.TLS{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Start(ctx)
	}()

	addr, ok := waitForListenerAddress(a, 2*time.Second)
	if !ok {
		cancel()
		t.Fatalf("listener did not start in time")
	}

	httpsClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test self-signed cert
		},
	}
	resp, err := httpsClient.Get("https://" + addr + "/health")
	if err != nil {
		cancel()
		t.Fatalf("https health request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("expected https /health 200, got %d", resp.StatusCode)
	}

	httpClient := &http.Client{Timeout: 2 * time.Second}
	plainResp, err := httpClient.Get("http://" + addr + "/health")
	if err == nil {
		_ = plainResp.Body.Close()
		if plainResp.StatusCode == http.StatusOK {
			cancel()
			t.Fatalf("expected plain http not to succeed on tls listener")
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("api start returned error after shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("api did not stop in time")
	}
}

func waitForListenerAddress(a *API, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a != nil {
			if addr := a.ListenerAddr(); addr != "" {
				return addr, true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", false
}

func writeSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:         false,
		SubjectKeyId: []byte{1, 2, 3, 4},
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "server.crt")
	keyFile = filepath.Join(dir, "server.key")

	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		_ = certOut.Close()
		t.Fatalf("encode cert: %v", err)
	}
	if err := certOut.Close(); err != nil {
		t.Fatalf("close cert file: %v", err)
	}

	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		_ = keyOut.Close()
		t.Fatalf("encode key: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		t.Fatalf("close key file: %v", err)
	}

	return certFile, keyFile
}

type fakeClusterHealthProvider struct {
	all map[uint64]*inner_cluster.ClusterHealthInfo
}

func (f fakeClusterHealthProvider) GetAllHealthStatus() map[uint64]*inner_cluster.ClusterHealthInfo {
	return f.all
}

func (f fakeClusterHealthProvider) GetHealthStatus(clusterID uint64) (*inner_cluster.ClusterHealthInfo, bool) {
	v, ok := f.all[clusterID]
	return v, ok
}

type fakeClusterOverviewProvider struct {
	overview *ClusterOverview
	err      error
}

func (f fakeClusterOverviewProvider) GetClusterOverview(context.Context) (*ClusterOverview, error) {
	return f.overview, f.err
}

func TestClusterRoutesDisabledWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/health", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cluster route disabled (404), got %d", rec.Code)
	}
}

func TestClusterRoutesReturnSummaryAndDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	now := time.Now()
	a := NewAPI(":0", &Component{
		ClusterHealth: fakeClusterHealthProvider{
			all: map[uint64]*inner_cluster.ClusterHealthInfo{
				2: {
					ClusterID:    2,
					ClusterName:  "key_store",
					Status:       inner_cluster.HealthStatusHealthy,
					LastCheck:    now,
					LastSuccess:  now,
					FailureCount: 0,
				},
				4: {
					ClusterID:    4,
					ClusterName:  "will_delay",
					Status:       inner_cluster.HealthStatusUnhealthy,
					LastCheck:    now,
					LastSuccess:  now.Add(-time.Minute),
					FailureCount: 3,
					Error:        "timeout",
				},
			},
		},
	})
	a.httpServer = gin.New()
	a.route()

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/health", nil)
	summaryRec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(summaryRec, summaryReq)
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("expected summary status 200, got %d", summaryRec.Code)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/health/4", nil)
	detailRec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d", detailRec.Code)
	}
}

func TestClusterOverviewRouteEnabledWithoutHealthProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		ClusterOverview: fakeClusterOverviewProvider{
			overview: &ClusterOverview{
				ClusterEnabled: false,
				RaftGroups:     nil,
				DeliveryBacklog: ClusterDeliveryBacklogOverview{
					Supported: false,
					Reason:    "cluster is disabled",
				},
			},
		},
	})
	a.httpServer = gin.New()
	a.route()

	overviewReq := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/overview", nil)
	overviewRec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("expected overview status 200, got %d", overviewRec.Code)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/health", nil)
	healthRec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusNotFound {
		t.Fatalf("expected health route disabled (404), got %d", healthRec.Code)
	}
}

func TestClusterOverviewRouteReturnsBacklogPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		ClusterOverview: fakeClusterOverviewProvider{
			overview: &ClusterOverview{
				ClusterEnabled: true,
				RaftGroups: []ClusterRaftGroupOverview{
					{
						ClusterID:         2,
						ClusterName:       "key_store",
						Kind:              "system",
						HasLeader:         true,
						LeaderNodeID:      1,
						Health:            "healthy",
						ReplicationStatus: "healthy",
					},
				},
				DeliveryBacklog: ClusterDeliveryBacklogOverview{
					Supported:     true,
					PendingTasks:  12,
					ActiveClients: 3,
				},
			},
		},
	})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/overview", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected overview status 200, got %d", rec.Code)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			DeliveryBacklog struct {
				Supported     bool  `json:"supported"`
				PendingTasks  int64 `json:"pending_tasks"`
				ActiveClients int64 `json:"active_clients"`
			} `json:"delivery_backlog"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal overview response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success=true, got false")
	}
	if !body.Data.DeliveryBacklog.Supported {
		t.Fatalf("expected backlog supported=true")
	}
	if body.Data.DeliveryBacklog.PendingTasks != 12 || body.Data.DeliveryBacklog.ActiveClients != 3 {
		t.Fatalf("unexpected backlog summary: %+v", body.Data.DeliveryBacklog)
	}
}

func TestClusterOverviewRouteReturns500WhenProviderFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		ClusterOverview: fakeClusterOverviewProvider{
			err: errors.New("boom"),
		},
	})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/overview", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected overview status 500, got %d", rec.Code)
	}
}
