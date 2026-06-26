package config

import "time"

func DefaultAppConfig() AppConfig {
	return AppConfig{
		Server:   DefaultServerConfig(),
		Broker:   DefaultBrokerConfig(),
		Storage:  DefaultStoreConfig(),
		Cluster:  DefaultClusterConfig(),
		Console:  DefaultConsoleConfig(),
		Delivery: DefaultDeliveryRunner(),
		Logging:  DefaultLogConfig(),
		Plugins:  GetDefaultPlugins(),
	}
}

func DefaultServerConfig() Server {
	return Server{
		Port: 9526,
		TLS:  DefaultTLSConfig(),
	}
}

func DefaultBrokerConfig() Broker {
	return Broker{
		Listen:                []string{"tcp://localhost:1883"},
		TLS:                   DefaultTLSConfig(),
		ConnectAckProperty:    DefaultConnectAckProperty(),
		LocalState:            DefaultLocalStateConfig(),
		NoSubTopicResponse:    0,
		KeepAlive:             180,
		KeepAliveScanInterval: time.Second,
		BatchReadSize:         200,
		MessageRetry:          DefaultMessageRetryConfig(),
		StoreQoS0:             true,
		ClientRateLimit:       DefaultClientRateLimitConfig(),
		Retain:                DefaultRetainConfig(),
		Limits:                DefaultBrokerLimits(),
		ACL:                   DefaultBrokerACLConfig(),
	}
}

func DefaultStoreConfig() Store {
	return Store{
		MessageExpired: 1,
		Default:        KeyStoreTypeBadger,
		DeliveryQueue: DeliveryQueueConfig{
			Type:           ClientDeliveryQueueTypeSingleNodeBadger,
			BucketDuration: time.Hour,
		},
		Payload: PayloadConfig{
			Type: ClientDeliveryPayloadTypeSingleNodeBadger,
		},
		Redis: Redis{
			Address:  "localhost:6379",
			Password: "",
			DB:       0,
		},
		Badger: Badger{
			Path: "./data/badger",
		},
		Cassandra: Cassandra{
			Hosts:           []string{"127.0.0.1"},
			Port:            9042,
			Keyspace:        "skytree",
			Consistency:     "LOCAL_QUORUM",
			Timeout:         3 * time.Second,
			ConnectTimeout:  3 * time.Second,
			NumConns:        2,
			AutoCreateTable: true,
		},
	}
}

func DefaultClusterConfig() Cluster {
	return Cluster{
		Enable:           false,
		Join:             false,
		DataDir:          "./data/cluster",
		LocalNodeAddress: "127.0.0.1:63001",
		LocalNodeID:      1,
		WriteTimeout:     5 * time.Second,
		GRPC: GRPC{
			Addr:          "0.0.0.0:53001",
			Endpoint:      "127.0.0.1:53001",
			AllowInsecure: false,
			TLS:           DefaultTLSConfig(),
		},
		HealthCheck: HealthCheck{
			Enabled:    false,
			Interval:   0,
			Timeout:    0,
			MaxRetries: 0,
		},
	}
}

func DefaultConsoleConfig() Console {
	return Console{
		Enabled:  false,
		Username: "",
		Password: "",
		Control: ConsoleControl{
			Enabled:  false,
			Provider: "gardener",
			BaseURL:  "",
			Token:    "",
			Timeout:  3 * time.Second,
			K8s: ConsoleK8sControl{
				APIServer:              "",
				Namespace:              "",
				LabelSelector:          "",
				ServiceName:            "skytree-headless",
				NodeIDAnnotation:       "skytree.io/node-id",
				RaftAddressAnnotation:  "skytree.io/raft-address",
				GRPCEndpointAnnotation: "skytree.io/grpc-endpoint",
				TokenFile:              "/var/run/secrets/kubernetes.io/serviceaccount/token",
				CAFile:                 "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
				RaftPort:               8080,
				GRPCPort:               8091,
				RequestTimeout:         3 * time.Second,
				JoinTimeout:            10 * time.Second,
			},
		},
	}
}

func DefaultLogConfig() Log {
	return Log{
		Level:            "info",
		File:             "",
		MaxSize:          100,
		MaxAge:           30,
		MaxBackups:       10,
		Compress:         true,
		GinMode:          "release",
		GinConsoleOutput: false,
		StartupReport:    true,
	}
}

func DefaultTLSConfig() TLS {
	return TLS{
		Enabled:        false,
		CertFile:       "",
		KeyFile:        "",
		CAFile:         "",
		ReloadInterval: 0,
		MTLSAuthMode:   "off",
	}
}

func DefaultConnectAckProperty() ConnectAckProperty {
	return ConnectAckProperty{
		ReceiveMaximum:                  65535,
		MaxQos:                          2,
		RetainAvailable:                 1,
		MaximumPacketSize:               1048576,
		TopicAliasMaximum:               10,
		WildcardSubscriptionAvailable:   true,
		SubscriptionIdentifierAvailable: true,
		SharedSubscriptionAvailable:     true,
		ServerKeepAlive:                 180,
		ResponseInformation:             "",
		ServerReference:                 "",
	}
}

func DefaultLocalStateConfig() LocalState {
	return LocalState{
		DataDir:          "./data/single",
		SnapshotInterval: 30 * time.Second,
		SnapshotEntries:  10000,
	}
}

func DefaultMessageRetryConfig() MessageRetry {
	return MessageRetry{
		MaxRetryCount:     3,
		Interval:          30 * time.Second,
		MaxTimeout:        300 * time.Second,
		SchedulerInterval: time.Second,
	}
}

func DefaultClientRateLimitConfig() ClientRateLimit {
	return ClientRateLimit{
		Enabled:           true,
		MessagesPerSecond: 10,
		WindowSize:        10,
	}
}

func DefaultRetainConfig() RetainConfig {
	return RetainConfig{
		GCInterval: 5 * time.Minute,
	}
}

func DefaultBrokerLimits() BrokerLimits {
	return BrokerLimits{
		WillDelayMaxSeconds:     604800,
		SessionExpiryMaxSeconds: 604800,
	}
}

func DefaultBrokerACLConfig() ACLConfig {
	return ACLConfig{
		QoS0RejectPolicy: "drop",
	}
}
