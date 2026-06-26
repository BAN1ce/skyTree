package config

import "time"

// Console configures the read-only web management console.
type Console struct {
	Enabled  bool           `yaml:"enabled" env:"CONSOLE_ENABLED" env-default:"false"`
	Username string         `yaml:"username" env:"CONSOLE_USERNAME" env-default:""`
	Password string         `yaml:"password" env:"CONSOLE_PASSWORD" env-default:""`
	Control  ConsoleControl `yaml:"control"`
}

type ConsoleControl struct {
	Enabled  bool              `yaml:"enabled" env:"CONSOLE_CONTROL_ENABLED" env-default:"false"`
	Provider string            `yaml:"provider" env:"CONSOLE_CONTROL_PROVIDER" env-default:"gardener"`
	BaseURL  string            `yaml:"base_url" env:"CONSOLE_CONTROL_BASE_URL" env-default:""`
	Token    string            `yaml:"token" env:"CONSOLE_CONTROL_TOKEN" env-default:""`
	Timeout  time.Duration     `yaml:"timeout" env:"CONSOLE_CONTROL_TIMEOUT" env-default:"3s"`
	K8s      ConsoleK8sControl `yaml:"k8s"`
}

type ConsoleK8sControl struct {
	APIServer              string        `yaml:"api_server" env:"CONSOLE_CONTROL_K8S_API_SERVER" env-default:""`
	Namespace              string        `yaml:"namespace" env:"CONSOLE_CONTROL_K8S_NAMESPACE" env-default:""`
	LabelSelector          string        `yaml:"label_selector" env:"CONSOLE_CONTROL_K8S_LABEL_SELECTOR" env-default:""`
	ServiceName            string        `yaml:"service_name" env:"CONSOLE_CONTROL_K8S_SERVICE_NAME" env-default:"skytree-headless"`
	NodeIDAnnotation       string        `yaml:"node_id_annotation" env:"CONSOLE_CONTROL_K8S_NODE_ID_ANNOTATION" env-default:"skytree.io/node-id"`
	RaftAddressAnnotation  string        `yaml:"raft_address_annotation" env:"CONSOLE_CONTROL_K8S_RAFT_ADDRESS_ANNOTATION" env-default:"skytree.io/raft-address"`
	GRPCEndpointAnnotation string        `yaml:"grpc_endpoint_annotation" env:"CONSOLE_CONTROL_K8S_GRPC_ENDPOINT_ANNOTATION" env-default:"skytree.io/grpc-endpoint"`
	TokenFile              string        `yaml:"token_file" env:"CONSOLE_CONTROL_K8S_TOKEN_FILE" env-default:"/var/run/secrets/kubernetes.io/serviceaccount/token"`
	CAFile                 string        `yaml:"ca_file" env:"CONSOLE_CONTROL_K8S_CA_FILE" env-default:"/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"`
	RaftPort               int           `yaml:"raft_port" env:"CONSOLE_CONTROL_K8S_RAFT_PORT" env-default:"8080"`
	GRPCPort               int           `yaml:"grpc_port" env:"CONSOLE_CONTROL_K8S_GRPC_PORT" env-default:"8091"`
	RequestTimeout         time.Duration `yaml:"request_timeout" env:"CONSOLE_CONTROL_K8S_REQUEST_TIMEOUT" env-default:"3s"`
	JoinTimeout            time.Duration `yaml:"join_timeout" env:"CONSOLE_CONTROL_K8S_JOIN_TIMEOUT" env-default:"10s"`
}
