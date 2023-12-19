package config

import "time"

type Broker struct {
	ConnectAckProperty
}

type ConnectAckProperty struct {
	SessionExpired                  string `json:"session_expired"`
	ReceiveMaximum                  int    `json:"receive_maximum"`
	MaxQos                          int    `json:"max_qos"`
	RetainAvailable                 bool   `json:"retain_available"`
	MaximumPacketSize               int    `json:"maximum_packet_size"`
	TopicAliasMaximum               int    `json:"topic_alias_maximum"`
	WildcardSubscriptionAvailable   bool   `json:"wildcard_subscription_available"`
	SubscriptionIdentifierAvailable bool   `json:"subscription_identifier_available"`
	SharedSubscriptionAvailable     bool   `json:"shared_subscription_available"`
	ServerKeepAlive                 int    `json:"server_keep_alive"`
	ResponseInformation             string `json:"response_information"`
	ServerReference                 string `json:"server_reference"`
	AuthenticationMethod            string `json:"authentication_method"`
	AuthenticationData              string `json:"authentication_data"`
}

type MessageRetry struct {
	MaxRetryCount int           `json:"max_retry_count"`
	Interval      time.Duration `json:"interval"`
	MaxTimeout    time.Duration `json:"max_timeout"`
}

type Store struct {
	Dir string
}
