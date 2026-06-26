package config

import "time"

// Plugins 插件配置
type Plugins struct {
	// Metric 插件配置
	Metric MetricPlugin `yaml:"metric"`

	// Auth 插件配置
	Auth AuthPlugin `yaml:"auth"`

	// ACL 插件配置
	ACL ACLPlugin `yaml:"acl"`

	// 自定义插件配置
	Custom []CustomPlugin `yaml:"custom"`
}

// MetricPlugin 监控插件配置
type MetricPlugin struct {
	Enabled bool `yaml:"enabled"` // 是否启用监控插件，默认启用
}

// AuthPlugin 认证插件配置
type AuthPlugin struct {
	Enabled bool       `yaml:"enabled"` // 是否启用认证插件，默认禁用
	Auth    AuthConfig `yaml:"auth"`    // AUTH报文处理配置
}

// AuthConfig AUTH报文处理配置
type AuthConfig struct {
	// Enabled 是否启用AUTH处理
	Enabled bool `yaml:"enabled"`

	// HTTPEndpoint HTTP认证服务地址（可选）
	HTTPEndpoint string `yaml:"http_endpoint"`

	// GRPCEndpoint gRPC认证服务地址（可选）
	GRPCEndpoint string `yaml:"grpc_endpoint"`

	// Timeout 超时时间（秒），默认5秒，最大10秒
	Timeout int `yaml:"timeout" default:"5"`
}

// ACLPlugin provides topic-level publish/subscribe authorization.
// Rules are loaded from a local file (preferred when exists) otherwise from KeyStore (Badger/Redis/etc).
type ACLPlugin struct {
	Enabled bool `yaml:"enabled"`

	// File is the ACL rules file path. When exists, it takes precedence and KeyStore is not read.
	File string `yaml:"file"`

	// KeyStoreKey is the key used to read ACL rules from KeyStore when File is absent.
	// The value is a YAML blob compatible with the file format.
	KeyStoreKey string `yaml:"keystore_key"`

	// DefaultDeny denies when no rule matches.
	DefaultDeny bool `yaml:"default_deny"`

	// ReloadInterval enables periodic reload when > 0.
	// Phase-2 feature; current implementation loads once at startup/hot path.
	ReloadInterval time.Duration `yaml:"reload_interval"`

	// AdminUsername and AdminPassword are used by HTTP ACL admin APIs basic auth.
	// Empty credentials disable ACL admin routes.
	AdminUsername string `yaml:"admin_username" env:"PLUGINS_ACL_ADMIN_USERNAME" env-default:""`
	AdminPassword string `yaml:"admin_password" env:"PLUGINS_ACL_ADMIN_PASSWORD" env-default:""`
}

// CustomPlugin 自定义插件配置
type CustomPlugin struct {
	Name    string                 `yaml:"name"`    // 插件名称
	Enabled bool                   `yaml:"enabled"` // 是否启用
	Config  map[string]interface{} `yaml:"config"`  // 插件配置
}

// GetDefaultPlugins 获取默认插件配置
func GetDefaultPlugins() Plugins {
	return Plugins{
		Metric: MetricPlugin{
			Enabled: true, // 默认启用监控插件
		},
		Auth: AuthPlugin{
			Enabled: false, // 默认禁用认证插件
			Auth: AuthConfig{
				Enabled: false,
				Timeout: 5, // 默认5秒
			},
		},
		ACL: ACLPlugin{
			Enabled:        false,
			DefaultDeny:    true,
			KeyStoreKey:    "acl:rules",
			ReloadInterval: 0,
			AdminUsername:  "",
			AdminPassword:  "",
		},
		Custom: []CustomPlugin{},
	}
}
