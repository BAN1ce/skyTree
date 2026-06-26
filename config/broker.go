package config

import "time"

type Broker struct {
	Listen []string `yaml:"listeners" env:"BROKER_LISTENERS" env-default:"tcp://localhost:1883"`

	// TLS is used by tls:// and wss:// listeners.
	TLS TLS `yaml:"tls"`

	ConnectAckProperty ConnectAckProperty `yaml:"connack"`

	// LocalState configures single-node persistence for in-memory states (session center, sub center).
	LocalState LocalState `yaml:"local_state"`

	NoSubTopicResponse byte `yaml:"no_sub_topic_behavior" env:"BROKER_NO_SUB_TOPIC_BEHAVIOR" env-default:"0"`

	KeepAlive int `yaml:"keep_alive_seconds" env:"BROKER_KEEP_ALIVE_SECONDS" env-default:"180"`

	KeepAliveScanInterval time.Duration `yaml:"keep_alive_scan_interval" env:"BROKER_KEEP_ALIVE_SCAN_INTERVAL" env-default:"1s"`

	BatchReadSize int `yaml:"read_batch_size" env:"BROKER_READ_BATCH_SIZE" env-default:"200"`

	MessageRetry MessageRetry `yaml:"message_retry"`

	StoreQoS0 bool `yaml:"persist_qos0" env:"BROKER_PERSIST_QOS0" env-default:"true"`

	ClientRateLimit ClientRateLimit `yaml:"client_rate"`

	Retain RetainConfig `yaml:"retain"`

	// Limits 是 broker 对 MQTT5 协议字段的额外上限约束，0 表示不约束。
	Limits BrokerLimits `yaml:"limits"`

	ACL ACLConfig `yaml:"acl"`
}

type ACLConfig struct {
	// QoS0RejectPolicy 控制 QoS0 PUBLISH 被 ACL 拒绝时的行为：
	//   "drop"       —— 静默丢弃（spec 推荐，默认）
	//   "disconnect" —— 立即下发 DISCONNECT(0x87) 并关闭连接
	QoS0RejectPolicy string `yaml:"qos0_reject_policy" env:"BROKER_ACL_QOS0_REJECT_POLICY" env-default:"drop"`
}

type BrokerLimits struct {
	// WillDelayMaxSeconds 限制客户端 CONNECT 中 Will Delay Interval 的最大值，单位秒。0 = 不限。
	WillDelayMaxSeconds uint32 `yaml:"will_delay_max_seconds" env:"BROKER_LIMITS_WILL_DELAY_MAX_SECONDS" env-default:"604800"`
	// SessionExpiryMaxSeconds 限制 CONNECT/DISCONNECT 中 Session Expiry Interval 的最大值，单位秒。0 = 不限。
	SessionExpiryMaxSeconds uint32 `yaml:"session_expiry_max_seconds" env:"BROKER_LIMITS_SESSION_EXPIRY_MAX_SECONDS" env-default:"604800"`
}

// RetainConfig 控制 retained 消息的后台清理（Message Expiry GC）。
type RetainConfig struct {
	// GCInterval 控制后台扫描清理已过期 retained 消息的时间间隔；
	// <=0 表示禁用后台 GC，仅依赖读路径上的 lazy 清理。
	GCInterval time.Duration `yaml:"gc_interval" env:"BROKER_RETAIN_GC_INTERVAL" env-default:"5m"`
}

type LocalState struct {
	// DataDir is the root directory where single-node state WAL/snapshots are stored.
	DataDir string `yaml:"data_dir" env:"BROKER_LOCAL_STATE_DATA_DIR" env-default:"./data/single"`
	// SnapshotInterval triggers a periodic full snapshot when >0.
	SnapshotInterval time.Duration `yaml:"snapshot_interval" env:"BROKER_LOCAL_STATE_SNAPSHOT_INTERVAL" env-default:"30s"`
	// SnapshotEntries triggers a full snapshot after this many writes since the last snapshot.
	SnapshotEntries uint64 `yaml:"snapshot_entries" env:"BROKER_LOCAL_STATE_SNAPSHOT_ENTRIES" env-default:"10000"`
}

type MessageRetry struct {
	MaxRetryCount     int           `yaml:"max_retry_count" env:"BROKER_MESSAGE_RETRY_MAX_RETRY_COUNT" env-default:"3"`
	Interval          time.Duration `yaml:"interval" env:"BROKER_MESSAGE_RETRY_INTERVAL" env-default:"30s"`
	MaxTimeout        time.Duration `yaml:"max_timeout" env:"BROKER_MESSAGE_RETRY_MAX_TIMEOUT" env-default:"300s"`
	SchedulerInterval time.Duration `yaml:"scheduler_interval" env:"BROKER_MESSAGE_RETRY_SCHEDULER_INTERVAL" env-default:"1s"`
}

type ClientRateLimit struct {
	// 是否启用客户端限流
	Enabled bool `yaml:"enabled" env:"BROKER_CLIENT_RATE_LIMIT_ENABLED" env-default:"true"`

	// 每秒最多接收的消息数，默认10
	MessagesPerSecond int `yaml:"messages_per_second" env:"BROKER_CLIENT_RATE_LIMIT_MESSAGES_PER_SECOND" env-default:"10"`

	// 限流窗口大小，用于平滑限流
	WindowSize int `yaml:"window_size" env:"BROKER_CLIENT_RATE_LIMIT_WINDOW_SIZE" env-default:"10"`
}
