package config

import "time"

type Cluster struct {
	Enable           bool              `yaml:"enable" env:"CLUSTER_ENABLE" env-default:"false"`
	Join             bool              `yaml:"join" env:"CLUSTER_JOIN" env-default:"false"`
	DataDir          string            `yaml:"data_dir" env:"CLUSTER_DATA_DIR" env-default:"./data/cluster"`
	LocalNodeAddress string            `yaml:"local_node_address" env:"CLUSTER_LOCAL_NODE_ADDRESS" env-default:"127.0.0.1:63001"`
	LocalNodeID      uint64            `yaml:"local_node_id" env:"CLUSTER_LOCAL_NODE_ID" env-default:"1"`
	Member           map[uint64]string `yaml:"member" env:"CLUSTER_MEMBER" env-default:""`
	WriteTimeout     time.Duration     `yaml:"write_timeout" env:"CLUSTER_WRITE_TIMEOUT" env-default:"5s"`
	GRPC             GRPC              `yaml:"grpc"`
	HealthCheck      HealthCheck       `yaml:"health_check"`
}

type GRPC struct {
	Addr     string `yaml:"addr" env:"CLUSTER_GRPC_ADDR" env-default:"0.0.0.0:53001"`
	Endpoint string `yaml:"endpoint" env:"CLUSTER_GRPC_ENDPOINT" env-default:"127.0.0.1:53001"`

	// AllowInsecure permits plaintext intra-cluster gRPC traffic when TLS is disabled.
	AllowInsecure bool `yaml:"allow_insecure" env:"CLUSTER_GRPC_ALLOW_INSECURE" env-default:"false"`

	// TLS enables optional gRPC server-side TLS.
	TLS TLS `yaml:"tls"`
}

type HealthCheck struct {
	Enabled    bool          `yaml:"enabled" env:"CLUSTER_HEALTH_CHECK_ENABLED" env-default:"false"`
	Interval   time.Duration `yaml:"interval" env:"CLUSTER_HEALTH_CHECK_INTERVAL" env-default:"0s"`
	Timeout    time.Duration `yaml:"timeout" env:"CLUSTER_HEALTH_CHECK_TIMEOUT" env-default:"0s"`
	MaxRetries int           `yaml:"max_retries" env:"CLUSTER_HEALTH_CHECK_MAX_RETRIES" env-default:"0"`
}
