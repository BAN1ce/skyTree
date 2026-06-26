package client

import (
	"reflect"
	"time"

	"github.com/BAN1ce/skyTree/config"
)

const defaultClientWriteTimeout = 5 * time.Second

func defaultBrokerRuntimeConfig() config.Broker {
	return config.Broker{
		KeepAlive: 180,
		ConnectAckProperty: config.ConnectAckProperty{
			ReceiveMaximum:                  65535,
			MaxQos:                          2,
			RetainAvailable:                 1,
			MaximumPacketSize:               1048576,
			TopicAliasMaximum:               10,
			WildcardSubscriptionAvailable:   true,
			SubscriptionIdentifierAvailable: true,
			SharedSubscriptionAvailable:     true,
			ServerKeepAlive:                 180,
		},
		ClientRateLimit: config.ClientRateLimit{
			Enabled:           true,
			MessagesPerSecond: 10,
			WindowSize:        10,
		},
		Limits: config.BrokerLimits{
			WillDelayMaxSeconds:     604800,
			SessionExpiryMaxSeconds: 604800,
		},
		ACL: config.ACLConfig{QoS0RejectPolicy: "drop"},
	}
}

func (c *Client) brokerRuntimeConfig() config.Broker {
	def := defaultBrokerRuntimeConfig()
	if c == nil || c.component == nil {
		return def
	}
	componentCfg := c.component.cfg
	if !componentCfg.provided || brokerConfigIsEmpty(componentCfg.BrokerConfig) {
		return def
	}
	cfg := componentCfg.BrokerConfig
	if !componentCfg.BrokerConfigResolved {
		cfg = mergePartialBrokerRuntimeConfig(def, cfg)
	}
	if cfg.ConnectAckProperty.ReceiveMaximum <= 0 {
		cfg.ConnectAckProperty.ReceiveMaximum = def.ConnectAckProperty.ReceiveMaximum
	}
	if cfg.ConnectAckProperty.MaxQos < 0 || cfg.ConnectAckProperty.MaxQos > 2 {
		cfg.ConnectAckProperty.MaxQos = def.ConnectAckProperty.MaxQos
	}
	if cfg.ConnectAckProperty.RetainAvailable != 0 && cfg.ConnectAckProperty.RetainAvailable != 1 {
		cfg.ConnectAckProperty.RetainAvailable = def.ConnectAckProperty.RetainAvailable
	}
	if cfg.ConnectAckProperty.MaximumPacketSize <= 0 {
		cfg.ConnectAckProperty.MaximumPacketSize = def.ConnectAckProperty.MaximumPacketSize
	}
	if cfg.ConnectAckProperty.TopicAliasMaximum < 0 {
		cfg.ConnectAckProperty.TopicAliasMaximum = def.ConnectAckProperty.TopicAliasMaximum
	}
	if cfg.ConnectAckProperty.ServerKeepAlive < 0 {
		cfg.ConnectAckProperty.ServerKeepAlive = def.ConnectAckProperty.ServerKeepAlive
	}
	if cfg.ClientRateLimit.MessagesPerSecond <= 0 {
		cfg.ClientRateLimit.MessagesPerSecond = def.ClientRateLimit.MessagesPerSecond
	}
	if cfg.ClientRateLimit.WindowSize <= 0 {
		cfg.ClientRateLimit.WindowSize = def.ClientRateLimit.WindowSize
	}
	if cfg.ACL.QoS0RejectPolicy == "" {
		cfg.ACL.QoS0RejectPolicy = def.ACL.QoS0RejectPolicy
	}
	return cfg
}

func brokerConfigIsEmpty(cfg config.Broker) bool {
	return reflect.DeepEqual(cfg, config.Broker{})
}

func mergePartialBrokerRuntimeConfig(def config.Broker, cfg config.Broker) config.Broker {
	merged := def
	if len(cfg.Listen) > 0 {
		merged.Listen = cfg.Listen
	}
	if cfg.NoSubTopicResponse != 0 {
		merged.NoSubTopicResponse = cfg.NoSubTopicResponse
	}
	if cfg.KeepAlive != 0 {
		merged.KeepAlive = cfg.KeepAlive
	}
	if cfg.ConnectAckProperty.ReceiveMaximum != 0 {
		merged.ConnectAckProperty.ReceiveMaximum = cfg.ConnectAckProperty.ReceiveMaximum
	}
	if cfg.ConnectAckProperty.MaxQos != 0 {
		merged.ConnectAckProperty.MaxQos = cfg.ConnectAckProperty.MaxQos
	}
	if cfg.ConnectAckProperty.RetainAvailable != 0 {
		merged.ConnectAckProperty.RetainAvailable = cfg.ConnectAckProperty.RetainAvailable
	}
	if cfg.ConnectAckProperty.MaximumPacketSize != 0 {
		merged.ConnectAckProperty.MaximumPacketSize = cfg.ConnectAckProperty.MaximumPacketSize
	}
	if cfg.ConnectAckProperty.TopicAliasMaximum != 0 {
		merged.ConnectAckProperty.TopicAliasMaximum = cfg.ConnectAckProperty.TopicAliasMaximum
	}
	if cfg.ConnectAckProperty.ServerKeepAlive != 0 {
		merged.ConnectAckProperty.ServerKeepAlive = cfg.ConnectAckProperty.ServerKeepAlive
	}
	if cfg.ClientRateLimit.MessagesPerSecond != 0 {
		merged.ClientRateLimit.MessagesPerSecond = cfg.ClientRateLimit.MessagesPerSecond
	}
	if cfg.ClientRateLimit.WindowSize != 0 {
		merged.ClientRateLimit.WindowSize = cfg.ClientRateLimit.WindowSize
	}
	if cfg.ACL.QoS0RejectPolicy != "" {
		merged.ACL.QoS0RejectPolicy = cfg.ACL.QoS0RejectPolicy
	}
	return merged
}

func (c *Client) writeTimeout() time.Duration {
	if c == nil || c.component == nil || c.component.cfg.WriteTimeout <= 0 {
		return defaultClientWriteTimeout
	}
	return c.component.cfg.WriteTimeout
}

func (c *Client) clusterRuntimeConfig() config.Cluster {
	if c == nil || c.component == nil {
		return config.Cluster{LocalNodeID: 1}
	}
	cfg := c.component.cfg.ClusterConfig
	if cfg.LocalNodeID == 0 {
		cfg.LocalNodeID = 1
	}
	return cfg
}

func (c *Client) deliveryRuntimeConfig() config.DeliveryRunner {
	if c == nil || c.component == nil {
		return config.DefaultDeliveryRunner()
	}
	cfg := c.component.cfg.DeliveryConfig
	if cfg == (config.DeliveryRunner{}) {
		return config.DefaultDeliveryRunner()
	}
	return cfg
}
