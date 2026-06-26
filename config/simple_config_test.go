package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}
	return path
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	configFile := writeTempConfig(t, `
server:
  port: 9526
broker:
  listeners:
    - tcp://127.0.0.1:1883
unknown_root: true
`)

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected config schema error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid config schema") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown_root") {
		t.Fatalf("expected unknown field in error, got: %v", err)
	}
}

func TestLoadRejectsLocalStateEnable(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
  local_state:
    enable: false
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected local_state.enable to be rejected")
	}
	if !strings.Contains(err.Error(), "enable") {
		t.Fatalf("expected error to include removed field, got: %v", err)
	}
}

func TestLoadAcceptsKnownFields(t *testing.T) {
	configFile := writeTempConfig(t, `
server:
  port: 9526
console:
  enabled: true
  username: admin
  password: secret
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	if _, err := Load(configFile); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestLoadRejectsLegacyRetryAndTimewheelFields(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
retry:
  interval: 5
timewheel:
  interval: "1s"
`)

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected legacy retry/timewheel fields to be rejected")
	}
	if !strings.Contains(err.Error(), "retry") && !strings.Contains(err.Error(), "timewheel") {
		t.Fatalf("expected legacy field name in error, got: %v", err)
	}
}

func TestLoadAcceptsMessageRetrySchedulerInterval(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
  message_retry:
    scheduler_interval: "250ms"
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if cfg.Broker.MessageRetry.SchedulerInterval.String() != "250ms" {
		t.Fatalf("scheduler_interval = %s, want 250ms", cfg.Broker.MessageRetry.SchedulerInterval)
	}
}

func TestLoadUsesCodeDefaultsForMinimalConfig(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
  badger:
    path: "./data/custom-badger"
`)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Server.Port != 9526 {
		t.Fatalf("server.port = %d, want 9526", cfg.Server.Port)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("logging.level = %q, want info", cfg.Logging.Level)
	}
	if cfg.Broker.KeepAlive != 180 {
		t.Fatalf("broker.keep_alive_seconds = %d, want 180", cfg.Broker.KeepAlive)
	}
	if !cfg.Plugins.Metric.Enabled {
		t.Fatal("expected metric plugin default to be enabled")
	}
	if cfg.Storage.Badger.Path != "./data/custom-badger" {
		t.Fatalf("storage.badger.path = %q, want custom path", cfg.Storage.Badger.Path)
	}
}

func TestLoadCapabilityNegativeConfig(t *testing.T) {
	cfg, err := Load("../etc/fixtures/config.capability-negative.yaml")
	if err != nil {
		t.Fatalf("load capability-negative config: %v", err)
	}
	if cfg.Broker.ConnectAckProperty.MaxQos != 1 {
		t.Fatalf("max_qos = %d, want 1", cfg.Broker.ConnectAckProperty.MaxQos)
	}
	if cfg.Broker.ConnectAckProperty.RetainAvailable != 0 {
		t.Fatalf("retain_available = %d, want 0", cfg.Broker.ConnectAckProperty.RetainAvailable)
	}
	if cfg.Broker.ConnectAckProperty.SubscriptionIdentifierAvailable {
		t.Fatal("subscription_identifier_available = true, want false")
	}
	if cfg.Broker.ConnectAckProperty.WildcardSubscriptionAvailable {
		t.Fatal("wildcard_subscription_available = true, want false")
	}
}

func TestLoadRejectsEnabledConsoleWithoutCredentials(t *testing.T) {
	configFile := writeTempConfig(t, `
console:
  enabled: true
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected console credentials validation error")
	}
	if !strings.Contains(err.Error(), "console.username is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAcceptsStartupOutputFields(t *testing.T) {
	configFile := writeTempConfig(t, `
server:
  port: 9526
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
logging:
  gin_mode: debug
  gin_console_output: true
  startup_report: false
`)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Logging.GinMode != "debug" {
		t.Fatalf("expected gin_mode to load as debug, got %q", cfg.Logging.GinMode)
	}
	if !cfg.Logging.GinConsoleOutput {
		t.Fatalf("expected gin_console_output to load as true")
	}
	if cfg.Logging.StartupReport {
		t.Fatalf("expected startup_report to load as false")
	}
}

func TestLoadWhitelistedEnvOverridesYAML(t *testing.T) {
	configFile := writeTempConfig(t, `
server:
  port: 9526
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	if err := os.Setenv("SERVER_PORT", "18888"); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	defer os.Unsetenv("SERVER_PORT")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Server.Port != 18888 {
		t.Fatalf("expected env override server port to 18888, got %d", cfg.Server.Port)
	}
}

func TestLoadIgnoresNonWhitelistedEnvOverride(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	if err := os.Setenv("STORAGE_DELIVERY_QUEUE_DRIVER", "scylla"); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	defer os.Unsetenv("STORAGE_DELIVERY_QUEUE_DRIVER")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Storage.DeliveryQueue.Type != ClientDeliveryQueueTypeSingleNodeBadger {
		t.Fatalf("delivery_queue.driver = %q, want YAML value %q", cfg.Storage.DeliveryQueue.Type, ClientDeliveryQueueTypeSingleNodeBadger)
	}
}

func TestLoadAppliesK8sOrdinalClusterDefaults(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://0.0.0.0:1883
storage:
  delivery_queue:
    driver: scylla
  payload:
    driver: scylla
  cassandra:
    hosts: ["scylla"]
cluster:
  enable: true
  join: false
  local_node_id: 1
  local_node_address: "${NODE_NAME}:8080"
  member:
    1: "skytree-0.skytree-headless.skytree-local.svc.cluster.local:8080"
    2: "skytree-1.skytree-headless.skytree-local.svc.cluster.local:8080"
    3: "skytree-2.skytree-headless.skytree-local.svc.cluster.local:8080"
  grpc:
    addr: "0.0.0.0:8091"
    endpoint: "${NODE_NAME}:8091"
    allow_insecure: true
`)
	t.Setenv("SKYTREE_K8S_AUTO_NODE_ID", "true")
	t.Setenv("POD_NAME", "skytree-3")
	t.Setenv("POD_NAMESPACE", "skytree-local")
	t.Setenv("SKYTREE_K8S_HEADLESS_SERVICE", "skytree-headless")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Cluster.LocalNodeID != 4 {
		t.Fatalf("LocalNodeID = %d, want 4", cfg.Cluster.LocalNodeID)
	}
	if !cfg.Cluster.Join {
		t.Fatalf("Join = false, want true for node outside initial members")
	}
	wantNodeName := "skytree-3.skytree-headless.skytree-local.svc.cluster.local"
	if cfg.Cluster.LocalNodeAddress != wantNodeName+":8080" {
		t.Fatalf("LocalNodeAddress = %q", cfg.Cluster.LocalNodeAddress)
	}
	if cfg.Cluster.GRPC.Endpoint != wantNodeName+":8091" {
		t.Fatalf("GRPC.Endpoint = %q", cfg.Cluster.GRPC.Endpoint)
	}
}

func TestLoadFailsWhenConfigPathDoesNotExist(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-config.yaml")

	_, err := Load(missingPath)
	if err == nil {
		t.Fatal("expected error for missing config file path")
	}
	if !strings.Contains(err.Error(), missingPath) {
		t.Fatalf("expected error to include missing path %q, got: %v", missingPath, err)
	}
}

func TestLoadAllowsDefaultsWhenConfigPathEmpty(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected defaults-only mode to work when config path is empty: %v", err)
	}
	if cfg.Broker.Listen[0] != "tcp://localhost:1883" {
		t.Fatalf("default broker listener = %q, want tcp://localhost:1883", cfg.Broker.Listen[0])
	}
	if cfg.Storage.DeliveryQueue.Type != ClientDeliveryQueueTypeSingleNodeBadger {
		t.Fatalf("default delivery_queue.driver = %q, want %q", cfg.Storage.DeliveryQueue.Type, ClientDeliveryQueueTypeSingleNodeBadger)
	}
}

func TestLoadRejectsWhenBrokerListenersMissing(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners: []
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected empty broker.listeners to fail")
	}
	if !strings.Contains(err.Error(), "broker.listeners must contain at least one non-empty listener") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadKeepsExplicitBrokerListeners(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
`)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(cfg.Broker.Listen) != 1 {
		t.Fatalf("expected one explicit listener, got %d", len(cfg.Broker.Listen))
	}
	if cfg.Broker.Listen[0] != "tcp://127.0.0.1:1883" {
		t.Fatalf("unexpected listener: %q", cfg.Broker.Listen[0])
	}
}

func TestBetaExampleConfigLoadsAndPassesBetaProfile(t *testing.T) {
	cfg, err := Load("../etc/examples/config.beta.example.yaml")
	if err != nil {
		t.Fatalf("beta example config should load: %v", err)
	}
	if err := ValidateForProfile(cfg, ValidationProfileBeta); err != nil {
		t.Fatalf("beta example config should satisfy beta profile: %v", err)
	}
}

func TestClusterExampleConfigLoads(t *testing.T) {
	if _, err := Load("../etc/examples/config.cluster.example.yaml"); err != nil {
		t.Fatalf("cluster example config should load: %v", err)
	}
}

func TestStressExampleConfigLoads(t *testing.T) {
	if _, err := Load("../etc/examples/config.stress.example.yaml"); err != nil {
		t.Fatalf("stress example config should load: %v", err)
	}
}

func TestLoadPreservesExplicitFalseAndZeroValues(t *testing.T) {
	configFile := writeTempConfig(t, `
broker:
  listeners:
    - tcp://127.0.0.1:1883
  connack:
    retain_available: 0
    wildcard_subscription_available: false
  client_rate:
    enabled: false
storage:
  delivery_queue:
    driver: single_node_badger
  payload:
    driver: single_node_badger
logging:
  startup_report: false
`)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Broker.ConnectAckProperty.RetainAvailable != 0 {
		t.Fatalf("retain_available = %d, want 0", cfg.Broker.ConnectAckProperty.RetainAvailable)
	}
	if cfg.Broker.ConnectAckProperty.WildcardSubscriptionAvailable {
		t.Fatal("wildcard_subscription_available = true, want false")
	}
	if cfg.Broker.ClientRateLimit.Enabled {
		t.Fatal("client_rate.enabled = true, want false")
	}
	if cfg.Logging.StartupReport {
		t.Fatal("logging.startup_report = true, want false")
	}
}
