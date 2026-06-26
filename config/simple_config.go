package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads configuration from YAML and a minimal env override allowlist.
//
// Precedence:
// 1. Code defaults
// 2. YAML file values
// 3. Minimal environment overrides
func Load(configFile string) (AppConfig, error) {
	cfg := DefaultAppConfig()

	if configFile != "" {
		if _, err := os.Stat(configFile); err != nil {
			return AppConfig{}, fmt.Errorf("config file %q is not accessible: %w", configFile, err)
		}
		if err := overlayConfigFromYAML(configFile, &cfg); err != nil {
			return AppConfig{}, fmt.Errorf("invalid config schema: %w", err)
		}
	}

	if err := ApplyEnvOverrides(&cfg); err != nil {
		return AppConfig{}, fmt.Errorf("failed to apply env config: %w", err)
	}

	expandEnvInPlace(&cfg)
	if err := Validate(cfg); err != nil {
		return AppConfig{}, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func overlayConfigFromYAML(configFile string, cfg *AppConfig) error {
	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err := decoder.Decode(cfg); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	var extra interface{}
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("multiple YAML documents are not supported")
	} else if err != io.EOF {
		return err
	}

	return nil
}

// ApplyEnvOverrides applies a YAML-first, minimal allowlist of deployment-time overrides.
func ApplyEnvOverrides(cfg *AppConfig) error {
	if cfg == nil {
		return nil
	}

	var err error

	if cfg.Server.Port, err = envInt("SERVER_PORT", cfg.Server.Port); err != nil {
		return err
	}
	if cfg.Broker.Listen, err = envStringSlice("BROKER_LISTENERS", cfg.Broker.Listen); err != nil {
		return err
	}

	cfg.Console.Username = envString("CONSOLE_USERNAME", cfg.Console.Username)
	cfg.Console.Password = envString("CONSOLE_PASSWORD", cfg.Console.Password)

	cfg.Broker.TLS.CertFile = envString("BROKER_TLS_CERT_FILE", cfg.Broker.TLS.CertFile)
	cfg.Broker.TLS.KeyFile = envString("BROKER_TLS_KEY_FILE", cfg.Broker.TLS.KeyFile)
	cfg.Broker.TLS.CAFile = envString("BROKER_TLS_CA_FILE", cfg.Broker.TLS.CAFile)

	cfg.Server.TLS.CertFile = envString("SERVER_TLS_CERT_FILE", cfg.Server.TLS.CertFile)
	cfg.Server.TLS.KeyFile = envString("SERVER_TLS_KEY_FILE", cfg.Server.TLS.KeyFile)
	cfg.Server.TLS.CAFile = envString("SERVER_TLS_CA_FILE", cfg.Server.TLS.CAFile)

	if cfg.Cluster.Enable, err = envBool("CLUSTER_ENABLE", cfg.Cluster.Enable); err != nil {
		return err
	}
	if cfg.Cluster.LocalNodeID, err = envUint64("CLUSTER_LOCAL_NODE_ID", cfg.Cluster.LocalNodeID); err != nil {
		return err
	}
	cfg.Cluster.LocalNodeAddress = envString("CLUSTER_LOCAL_NODE_ADDRESS", cfg.Cluster.LocalNodeAddress)
	cfg.Cluster.GRPC.Addr = envString("CLUSTER_GRPC_ADDR", cfg.Cluster.GRPC.Addr)
	cfg.Cluster.GRPC.Endpoint = envString("CLUSTER_GRPC_ENDPOINT", cfg.Cluster.GRPC.Endpoint)
	cfg.Cluster.GRPC.TLS.CertFile = envString("CLUSTER_GRPC_TLS_CERT_FILE", cfg.Cluster.GRPC.TLS.CertFile)
	cfg.Cluster.GRPC.TLS.KeyFile = envString("CLUSTER_GRPC_TLS_KEY_FILE", cfg.Cluster.GRPC.TLS.KeyFile)
	cfg.Cluster.GRPC.TLS.CAFile = envString("CLUSTER_GRPC_TLS_CA_FILE", cfg.Cluster.GRPC.TLS.CAFile)

	if cfg.Storage.Cassandra.Hosts, err = envStringSlice("STORAGE_CASSANDRA_HOSTS", cfg.Storage.Cassandra.Hosts); err != nil {
		return err
	}
	if cfg.Storage.Cassandra.Port, err = envInt("STORAGE_CASSANDRA_PORT", cfg.Storage.Cassandra.Port); err != nil {
		return err
	}
	cfg.Storage.Cassandra.Keyspace = envString("STORAGE_CASSANDRA_KEYSPACE", cfg.Storage.Cassandra.Keyspace)
	cfg.Storage.Cassandra.Username = envString("STORAGE_CASSANDRA_USERNAME", cfg.Storage.Cassandra.Username)
	cfg.Storage.Cassandra.Password = envString("STORAGE_CASSANDRA_PASSWORD", cfg.Storage.Cassandra.Password)

	return nil
}

func envString(name, current string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return current
}

func envBool(name string, current bool) (bool, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return current, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return current, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func envInt(name string, current int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return current, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return current, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func envUint64(name string, current uint64) (uint64, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return current, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return current, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func envStringSlice(name string, current []string) ([]string, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return current, nil
	}
	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out, nil
}
