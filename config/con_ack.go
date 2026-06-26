package config

type ConnectAckProperty struct {
	ReceiveMaximum int `yaml:"receive_maximum" env:"BROKER_CONNACK_RECEIVE_MAXIMUM" env-default:"65535"`

	MaxQos int `yaml:"max_qos" env:"BROKER_CONNACK_MAX_QOS" env-default:"2"`

	RetainAvailable int `yaml:"retain_available" env:"BROKER_CONNACK_RETAIN_AVAILABLE" env-default:"1"`

	MaximumPacketSize int `yaml:"maximum_packet_size" env:"BROKER_CONNACK_MAXIMUM_PACKET_SIZE" env-default:"1048576"`

	TopicAliasMaximum int `yaml:"topic_alias_maximum" env:"BROKER_CONNACK_TOPIC_ALIAS_MAXIMUM" env-default:"10"`

	WildcardSubscriptionAvailable bool `yaml:"wildcard_subscription_available" env:"BROKER_CONNACK_WILDCARD_SUBSCRIPTION_AVAILABLE" env-default:"true"`

	SubscriptionIdentifierAvailable bool `yaml:"subscription_identifier_available" env:"BROKER_CONNACK_SUBSCRIPTION_IDENTIFIER_AVAILABLE" env-default:"true"`

	SharedSubscriptionAvailable bool `yaml:"shared_subscription_available" env:"BROKER_CONNACK_SHARED_SUBSCRIPTION_AVAILABLE" env-default:"true"`

	ServerKeepAlive int `yaml:"server_keep_alive" env:"BROKER_CONNACK_SERVER_KEEP_ALIVE" env-default:"180"`

	ResponseInformation string `yaml:"response_information" env:"BROKER_CONNACK_RESPONSE_INFORMATION" env-default:""`

	ServerReference string `yaml:"server_reference" env:"BROKER_CONNACK_SERVER_REFERENCE" env-default:""`
}
