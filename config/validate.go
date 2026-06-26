package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type ValidationProfile string

const (
	ValidationProfileDefault ValidationProfile = ""
	ValidationProfileBeta    ValidationProfile = "beta"
)

// expandEnvInPlace expands ${VAR} and $VAR in selected config fields.
// It is intentionally conservative (address-like fields only).
func expandEnvInPlace(cfg *AppConfig) {
	if cfg == nil {
		return
	}

	applyK8sOrdinalClusterDefaults(cfg)

	cfg.Cluster.DataDir = os.ExpandEnv(cfg.Cluster.DataDir)
	cfg.Cluster.LocalNodeAddress = os.ExpandEnv(cfg.Cluster.LocalNodeAddress)
	cfg.Cluster.GRPC.Addr = os.ExpandEnv(cfg.Cluster.GRPC.Addr)
	cfg.Cluster.GRPC.Endpoint = os.ExpandEnv(cfg.Cluster.GRPC.Endpoint)

	if cfg.Cluster.Member != nil {
		for id, addr := range cfg.Cluster.Member {
			cfg.Cluster.Member[id] = os.ExpandEnv(addr)
		}
	}

	for i, l := range cfg.Broker.Listen {
		cfg.Broker.Listen[i] = os.ExpandEnv(l)
	}
}

func applyK8sOrdinalClusterDefaults(cfg *AppConfig) {
	if !k8sAutoNodeIDEnabled() {
		return
	}
	podName := strings.TrimSpace(os.Getenv("POD_NAME"))
	if podName == "" {
		podName = strings.TrimSpace(os.Getenv("HOSTNAME"))
	}
	ordinal, ok := statefulSetOrdinal(podName)
	if !ok {
		return
	}
	nodeID := uint64(ordinal + 1)
	cfg.Cluster.LocalNodeID = nodeID
	cfg.Cluster.Join = !clusterMemberContainsNode(cfg.Cluster.Member, nodeID)

	namespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE"))
	if namespace == "" {
		namespace = "default"
	}
	serviceName := strings.TrimSpace(os.Getenv("SKYTREE_K8S_HEADLESS_SERVICE"))
	if serviceName == "" {
		serviceName = "skytree-headless"
	}
	nodeName := fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, serviceName, namespace)
	if err := os.Setenv("NODE_NAME", nodeName); err != nil {
		return
	}
}

func k8sAutoNodeIDEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SKYTREE_K8S_AUTO_NODE_ID")))
	return value == "true" || value == "1" || value == "yes"
}

func statefulSetOrdinal(name string) (int, bool) {
	index := strings.LastIndex(name, "-")
	if index < 0 || index == len(name)-1 {
		return 0, false
	}
	ordinal, err := strconv.Atoi(name[index+1:])
	if err != nil || ordinal < 0 {
		return 0, false
	}
	return ordinal, true
}

func clusterMemberContainsNode(members map[uint64]string, nodeID uint64) bool {
	_, ok := members[nodeID]
	return ok
}

// Validate validates the complete startup configuration.
func Validate(cfg AppConfig) error {
	return ValidateForProfile(cfg, ValidationProfileDefault)
}

// ValidateForProfile validates the complete startup configuration and applies
// optional policy checks for specific release profiles such as beta.
func ValidateForProfile(cfg AppConfig, profile ValidationProfile) error {
	if err := validateTLSConfig(cfg); err != nil {
		return err
	}
	if err := validateConsoleConfig(cfg.Console); err != nil {
		return err
	}
	if err := validateStorageDriver(cfg.Storage.Default); err != nil {
		return err
	}
	if err := validateClusterConfig(cfg.Cluster); err != nil {
		return err
	}
	if err := validateMessageRetryConfig(cfg.Broker.MessageRetry); err != nil {
		return err
	}

	if _, err := cfg.ResolveClientDeliverySpec(); err != nil {
		return err
	}
	if err := validateProfilePolicy(cfg, profile); err != nil {
		return err
	}

	return nil
}

func validateConsoleConfig(cfg Console) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return fmt.Errorf("console.username is required when console.enabled=true")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return fmt.Errorf("console.password is required when console.enabled=true")
	}
	if cfg.Control.Enabled {
		provider := strings.TrimSpace(cfg.Control.Provider)
		if provider == "" {
			return fmt.Errorf("console.control.provider is required when console.control.enabled=true")
		}
		switch provider {
		case "gardener":
			if strings.TrimSpace(cfg.Control.BaseURL) == "" {
				return fmt.Errorf("console.control.base_url is required when console.control.enabled=true")
			}
		case "k8s":
			if err := validateConsoleK8sConfig(cfg.Control.K8s); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported console.control.provider %q", provider)
		}
		if cfg.Control.Timeout <= 0 {
			return fmt.Errorf("console.control.timeout must be positive")
		}
	}
	return nil
}

func validateConsoleK8sConfig(cfg ConsoleK8sControl) error {
	if strings.TrimSpace(cfg.Namespace) == "" {
		return fmt.Errorf("console.control.k8s.namespace is required when provider=k8s")
	}
	if strings.TrimSpace(cfg.LabelSelector) == "" {
		return fmt.Errorf("console.control.k8s.label_selector is required when provider=k8s")
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return fmt.Errorf("console.control.k8s.service_name is required when provider=k8s")
	}
	if cfg.RaftPort <= 0 {
		return fmt.Errorf("console.control.k8s.raft_port must be positive")
	}
	if cfg.GRPCPort <= 0 {
		return fmt.Errorf("console.control.k8s.grpc_port must be positive")
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("console.control.k8s.request_timeout must be positive")
	}
	if cfg.JoinTimeout <= 0 {
		return fmt.Errorf("console.control.k8s.join_timeout must be positive")
	}
	return nil
}

func validateStorageDriver(driver string) error {
	switch strings.TrimSpace(driver) {
	case "":
		return nil
	case KeyStoreTypeBadger, KeyStoreTypeRedis:
		return nil
	default:
		return fmt.Errorf(
			"unsupported storage.driver %q (supported: %s, %s)",
			driver,
			KeyStoreTypeBadger,
			KeyStoreTypeRedis,
		)
	}
}

func validateMessageRetryConfig(cfg MessageRetry) error {
	if cfg.MaxRetryCount <= 0 {
		return fmt.Errorf("broker.message_retry.max_retry_count must be positive")
	}
	if cfg.Interval <= 0 {
		return fmt.Errorf("broker.message_retry.interval must be positive")
	}
	if cfg.MaxTimeout <= 0 {
		return fmt.Errorf("broker.message_retry.max_timeout must be positive")
	}
	if cfg.SchedulerInterval <= 0 {
		return fmt.Errorf("broker.message_retry.scheduler_interval must be positive")
	}
	return nil
}

func validateTLSConfig(cfg AppConfig) error {
	needsBrokerTLS := false
	nonEmptyListeners := 0
	for _, l := range cfg.Broker.Listen {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		nonEmptyListeners++
		proto, _, err := getProtocolAndAddressSafe(l)
		if err != nil {
			return err
		}
		switch proto {
		case "tcp", "ws", "tls", "wss":
		default:
			return fmt.Errorf("unsupported broker listener protocol %q in %q (supported: tcp, ws, tls, wss)", proto, l)
		}
		if proto == "tls" || proto == "wss" {
			needsBrokerTLS = true
			break
		}
	}
	if nonEmptyListeners == 0 {
		return fmt.Errorf("broker.listeners must contain at least one non-empty listener")
	}
	if needsBrokerTLS {
		if !cfg.Broker.TLS.Enabled {
			return fmt.Errorf("broker.tls.enabled must be true when using tls:// or wss:// listeners")
		}
		if err := validateTLSBlock("broker.tls", cfg.Broker.TLS, true); err != nil {
			return err
		}
	}

	if cfg.Server.TLS.Enabled {
		if err := validateTLSBlock("server.tls", cfg.Server.TLS, true); err != nil {
			return err
		}
	}

	if cfg.Cluster.Enable && cfg.Cluster.GRPC.TLS.Enabled {
		if err := validateTLSBlock("cluster.grpc.tls", cfg.Cluster.GRPC.TLS, true); err != nil {
			return err
		}
	}

	if err := validateConnectAckProperty(cfg.Broker.ConnectAckProperty); err != nil {
		return err
	}

	return nil
}

func validateClusterConfig(cfg Cluster) error {
	if !cfg.Enable {
		return nil
	}
	if cfg.LocalNodeID == 0 {
		return fmt.Errorf("cluster.local_node_id must be greater than 0")
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
		return fmt.Errorf("cluster.data_dir is required when cluster.enable=true")
	}
	if strings.TrimSpace(cfg.LocalNodeAddress) == "" {
		return fmt.Errorf("cluster.local_node_address is required when cluster.enable=true")
	}
	if err := validateHostPort("cluster.local_node_address", cfg.LocalNodeAddress); err != nil {
		return err
	}
	if len(cfg.Member) == 0 {
		return fmt.Errorf("cluster.member is required when cluster.enable=true")
	}
	if _, ok := cfg.Member[cfg.LocalNodeID]; !ok && !cfg.Join {
		return fmt.Errorf("cluster.member must contain cluster.local_node_id (%d) when cluster.join=false", cfg.LocalNodeID)
	}
	for id, addr := range cfg.Member {
		if err := validateHostPort(fmt.Sprintf("cluster.member[%d]", id), addr); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.GRPC.Addr) == "" {
		return fmt.Errorf("cluster.grpc.addr is required when cluster.enable=true")
	}
	if err := validateHostPort("cluster.grpc.addr", cfg.GRPC.Addr); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.GRPC.Endpoint) != "" {
		if err := validateHostPort("cluster.grpc.endpoint", cfg.GRPC.Endpoint); err != nil {
			return err
		}
	}
	if !cfg.GRPC.TLS.Enabled && !cfg.GRPC.AllowInsecure {
		return fmt.Errorf("cluster.grpc.tls.enabled must be true when cluster.enable=true (or explicitly set cluster.grpc.allow_insecure=true)")
	}
	return nil
}

func validateTLSBlock(field string, cfg TLS, requireServerCert bool) error {
	if !cfg.Enabled {
		return nil
	}
	if requireServerCert {
		if strings.TrimSpace(cfg.CertFile) == "" || strings.TrimSpace(cfg.KeyFile) == "" {
			return fmt.Errorf("%s.cert_file and %s.key_file are required when %s.enabled=true", field, field, field)
		}
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.MTLSAuthMode))
	switch mode {
	case "", "off":
		return nil
	case "optional", "required":
		if strings.TrimSpace(cfg.CAFile) == "" {
			return fmt.Errorf("%s.ca_file is required when %s.mtls_auth_mode=%s", field, field, mode)
		}
		return nil
	default:
		return fmt.Errorf("%s.mtls_auth_mode must be one of: off, optional, required", field)
	}
}

func validateProfilePolicy(cfg AppConfig, profile ValidationProfile) error {
	switch profile {
	case ValidationProfileDefault:
		return nil
	case ValidationProfileBeta:
		if cfg.Cluster.Enable && cfg.Cluster.GRPC.AllowInsecure {
			return fmt.Errorf("cluster.grpc.allow_insecure must be false for beta profile")
		}
		return nil
	default:
		return fmt.Errorf("unsupported validation profile %q", profile)
	}
}

// validateConnectAckProperty validates broker.connack for MQTT 5.0 CONNACK.
func validateConnectAckProperty(p ConnectAckProperty) error {
	if p.MaxQos < 0 || p.MaxQos > 2 {
		return fmt.Errorf("broker.connack.max_qos must be 0, 1, or 2, got %d", p.MaxQos)
	}
	if p.ReceiveMaximum < 1 || p.ReceiveMaximum > 65535 {
		return fmt.Errorf("broker.connack.receive_maximum must be 1..65535, got %d", p.ReceiveMaximum)
	}
	if p.RetainAvailable != 0 && p.RetainAvailable != 1 {
		return fmt.Errorf("broker.connack.retain_available must be 0 or 1, got %d", p.RetainAvailable)
	}
	if p.MaximumPacketSize < 1 || p.MaximumPacketSize > 268435455 {
		return fmt.Errorf("broker.connack.maximum_packet_size must be 1..268435455, got %d", p.MaximumPacketSize)
	}
	if p.TopicAliasMaximum < 0 || p.TopicAliasMaximum > 65535 {
		return fmt.Errorf("broker.connack.topic_alias_maximum must be 0..65535, got %d", p.TopicAliasMaximum)
	}
	if p.ServerKeepAlive < 0 || p.ServerKeepAlive > 65535 {
		return fmt.Errorf("broker.connack.server_keep_alive must be 0..65535, got %d", p.ServerKeepAlive)
	}
	return nil
}

func getProtocolAndAddressSafe(address string) (string, string, error) {
	parts := strings.SplitN(address, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid broker.listeners address format: %s", address)
	}
	return parts[0], parts[1], nil
}

func validateHostPort(field, v string) error {
	s := strings.TrimSpace(v)
	if s == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if strings.HasPrefix(s, "/") {
		return fmt.Errorf("%s must be a reachable address in host:port form, got %q", field, s)
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("%s must be in host:port form, got %q: %w", field, s, err)
	}
	_ = host
	if strings.TrimSpace(port) == "" {
		return fmt.Errorf("%s must have a port, got %q", field, s)
	}
	return nil
}
