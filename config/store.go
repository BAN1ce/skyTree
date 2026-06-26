package config

import "time"

type Store struct {
	MessageExpired int    `yaml:"message_expire_days" env:"STORAGE_MESSAGE_EXPIRE_DAYS" env-default:"1"`
	Default        string `yaml:"driver" env:"STORAGE_DRIVER" env-default:"badger"`

	// DeliveryQueue configures the delivery queue metadata store (DeliveryQueueStore).
	DeliveryQueue DeliveryQueueConfig `yaml:"delivery_queue"`
	// Payload configures the message payload store (MessagePayloadStore).
	Payload PayloadConfig `yaml:"payload"`

	Redis     Redis     `yaml:"redis"`
	Badger    Badger    `yaml:"badger"`
	Cassandra Cassandra `yaml:"cassandra"`
}

type DeliveryQueueConfig struct {
	// Type selects the delivery metadata store implementation (tasks/cursors).
	// Shared-subscription metadata is persisted in the same metadata backend.
	// Supported values: "single_node_badger", "scylla".
	Type string `yaml:"driver" env:"STORAGE_DELIVERY_QUEUE_DRIVER"`

	// BucketDuration controls Scylla delivery queue partition bucket sizing.
	BucketDuration time.Duration `yaml:"bucket_duration" env:"STORAGE_DELIVERY_QUEUE_BUCKET_DURATION" env-default:"1h"`
}

type PayloadConfig struct {
	// Type selects the message payload store implementation (message bytes data plane).
	// Supported values: "single_node_badger", "scylla".
	Type string `yaml:"driver" env:"STORAGE_PAYLOAD_DRIVER"`

}

type Redis struct {
	// redis config
	Address        string        `yaml:"address" env:"STORAGE_REDIS_ADDRESS" env-default:"localhost:6379"`
	Password       string        `yaml:"password" env:"STORAGE_REDIS_PASSWORD" env-default:""`
	DB             int           `yaml:"db" env:"STORAGE_REDIS_DB" env-default:"0"`
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"STORAGE_REDIS_CONNECT_TIMEOUT" env-default:"0s"`
	IdleTimeout    time.Duration `yaml:"idle_timeout" env:"STORAGE_REDIS_IDLE_TIMEOUT" env-default:"0s"`
	MaxActive      int           `yaml:"max_active" env:"STORAGE_REDIS_MAX_ACTIVE" env-default:"0"`
	MaxIdle        int           `yaml:"max_idle" env:"STORAGE_REDIS_MAX_IDLE" env-default:"0"`
	ReadTimeout    time.Duration `yaml:"read_timeout" env:"STORAGE_REDIS_READ_TIMEOUT" env-default:"0s"`
	WriteTimeout   time.Duration `yaml:"write_timeout" env:"STORAGE_REDIS_WRITE_TIMEOUT" env-default:"0s"`
}

type Cassandra struct {
	Hosts []string `yaml:"hosts" env:"STORAGE_CASSANDRA_HOSTS" env-separator:"," env-default:"127.0.0.1"`
	Port  int      `yaml:"port" env:"STORAGE_CASSANDRA_PORT" env-default:"9042"`

	Keyspace string `yaml:"keyspace" env:"STORAGE_CASSANDRA_KEYSPACE" env-default:"skytree"`

	Username string `yaml:"username" env:"STORAGE_CASSANDRA_USERNAME" env-default:""`
	Password string `yaml:"password" env:"STORAGE_CASSANDRA_PASSWORD" env-default:""`

	Consistency string `yaml:"consistency" env:"STORAGE_CASSANDRA_CONSISTENCY" env-default:"LOCAL_QUORUM"`

	Timeout        time.Duration `yaml:"timeout" env:"STORAGE_CASSANDRA_TIMEOUT" env-default:"3s"`
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"STORAGE_CASSANDRA_CONNECT_TIMEOUT" env-default:"3s"`
	NumConns       int           `yaml:"num_conns" env:"STORAGE_CASSANDRA_NUM_CONNS" env-default:"2"`

	AutoCreateTable bool `yaml:"auto_create_table" env:"STORAGE_CASSANDRA_AUTO_CREATE_TABLE" env-default:"true"`
}

type Badger struct {
	Path string `yaml:"path" env:"STORAGE_BADGER_PATH" env-default:"./data/badger"`
}
