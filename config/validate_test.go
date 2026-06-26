package config

import (
	"strings"
	"testing"
)

func validConnectAckPropertyForValidate() ConnectAckProperty {
	return ConnectAckProperty{
		MaxQos:            2,
		ReceiveMaximum:    100,
		RetainAvailable:   1,
		MaximumPacketSize: 4096,
		TopicAliasMaximum: 10,
		ServerKeepAlive:   60,
	}
}

func validBrokerForValidate() Broker {
	return Broker{
		Listen:             []string{"tcp://127.0.0.1:1883"},
		ConnectAckProperty: validConnectAckPropertyForValidate(),
		MessageRetry:       DefaultMessageRetryConfig(),
	}
}

func validDeliveryStoreForValidate() Store {
	return Store{
		Default: KeyStoreTypeBadger,
		DeliveryQueue: DeliveryQueueConfig{
			Type: ClientDeliveryQueueTypeSingleNodeBadger,
		},
		Payload: PayloadConfig{
			Type: ClientDeliveryPayloadTypeSingleNodeBadger,
		},
	}
}

func validScyllaDeliveryStoreForValidate() Store {
	return Store{
		Default: KeyStoreTypeBadger,
		DeliveryQueue: DeliveryQueueConfig{
			Type: ClientDeliveryQueueTypeScylla,
		},
		Payload: PayloadConfig{
			Type: ClientDeliveryPayloadTypeScylla,
		},
	}
}

func TestValidateConnectAckProperty_Valid(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validDeliveryStoreForValidate(),
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error for valid config: %v", err)
	}
}

func TestValidateConnectAckProperty_MaxQosInvalid(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validDeliveryStoreForValidate(),
	}
	cfg.Broker.ConnectAckProperty.MaxQos = 3
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for max_qos 3")
	}
}

func TestValidateConnectAckProperty_ReceiveMaximumInvalid(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validDeliveryStoreForValidate(),
	}
	cfg.Broker.ConnectAckProperty.ReceiveMaximum = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for receive_maximum 0")
	}
}

func TestValidateRejectsInvalidMessageRetrySchedulerInterval(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validDeliveryStoreForValidate(),
	}
	cfg.Broker.MessageRetry.SchedulerInterval = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid scheduler interval to fail validation")
	}
}

func TestValidateRejectsMissingDeliveryQueueDriver(t *testing.T) {
	cfg := AppConfig{
		Broker: validBrokerForValidate(),
		Storage: Store{
			Default: KeyStoreTypeBadger,
			Payload: PayloadConfig{Type: ClientDeliveryPayloadTypeSingleNodeBadger},
		},
		Cluster: Cluster{Enable: false},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected missing delivery queue driver to fail")
	}
	if !strings.Contains(err.Error(), "storage.delivery_queue.driver is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsMissingPayloadDriver(t *testing.T) {
	cfg := AppConfig{
		Broker: validBrokerForValidate(),
		Storage: Store{
			Default: KeyStoreTypeBadger,
			DeliveryQueue: DeliveryQueueConfig{
				Type: ClientDeliveryQueueTypeSingleNodeBadger,
			},
		},
		Cluster: Cluster{Enable: false},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected missing payload driver to fail")
	}
	if !strings.Contains(err.Error(), "storage.payload.driver is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveClientDeliverySpecRejectsLegacyAndMixedDrivers(t *testing.T) {
	tests := []struct {
		name        string
		queueType   string
		payloadType string
	}{
		{
			name:        "legacy queue driver is rejected",
			queueType:   "local_badger",
			payloadType: ClientDeliveryPayloadTypeSingleNodeBadger,
		},
		{
			name:        "legacy payload driver is rejected",
			queueType:   ClientDeliveryQueueTypeSingleNodeBadger,
			payloadType: "cassandra",
		},
		{
			name:        "mixed queue/payload pair is rejected",
			queueType:   ClientDeliveryQueueTypeSingleNodeBadger,
			payloadType: ClientDeliveryPayloadTypeScylla,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppConfig{
				Storage: Store{
					Default: KeyStoreTypeRedis,
					DeliveryQueue: DeliveryQueueConfig{
						Type: tt.queueType,
					},
					Payload: PayloadConfig{
						Type: tt.payloadType,
					},
				},
				Cluster: Cluster{Enable: false},
			}
			if _, err := cfg.ResolveClientDeliverySpec(); err == nil {
				t.Fatalf("expected ResolveClientDeliverySpec to fail")
			}
		})
	}
}

func TestResolveClientDeliverySpecAllowsScyllaPairInStandaloneAndCluster(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := AppConfig{
			Storage: Store{
				Default: KeyStoreTypeRedis,
				DeliveryQueue: DeliveryQueueConfig{
					Type: ClientDeliveryQueueTypeScylla,
				},
				Payload: PayloadConfig{
					Type: ClientDeliveryPayloadTypeScylla,
				},
			},
			Cluster: Cluster{Enable: enabled},
		}
		spec, err := cfg.ResolveClientDeliverySpec()
		if err != nil {
			t.Fatalf("ResolveClientDeliverySpec error: %v", err)
		}
		if spec.QueueType != ClientDeliveryQueueTypeScylla || spec.PayloadType != ClientDeliveryPayloadTypeScylla {
			t.Fatalf("unexpected spec: %+v", spec)
		}
	}
}

func TestResolveClientDeliverySpecSingleNodeBadgerMode(t *testing.T) {
	valid := AppConfig{
		Storage: Store{
			Default: KeyStoreTypeBadger,
			DeliveryQueue: DeliveryQueueConfig{
				Type: ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: PayloadConfig{
				Type: ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: Cluster{Enable: false},
	}
	if _, err := valid.ResolveClientDeliverySpec(); err != nil {
		t.Fatalf("expected single-node badger pair to pass: %v", err)
	}

	invalid := valid
	invalid.Cluster.Enable = true
	if _, err := invalid.ResolveClientDeliverySpec(); err == nil {
		t.Fatal("expected single-node badger pair to fail when cluster.enable=true")
	}
}

func TestDefaultConfigUsesSingleNodeBadgerDeliveryStores(t *testing.T) {
	cfg, err := Load("../etc/config.yaml")
	if err != nil {
		t.Fatalf("default config should load: %v", err)
	}
	spec, err := cfg.ResolveClientDeliverySpec()
	if err != nil {
		t.Fatalf("resolve delivery spec: %v", err)
	}
	if cfg.Cluster.Enable {
		t.Fatal("this test expects etc/config.yaml to be the standalone default config")
	}
	if spec.QueueType != ClientDeliveryQueueTypeSingleNodeBadger || spec.PayloadType != ClientDeliveryPayloadTypeSingleNodeBadger {
		t.Fatalf("standalone default config must not require external raft/cassandra stores, got %+v", spec)
	}
}

func TestValidateRejectsClusterInsecureGRPCByDefault(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "/tmp/skytree",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:          "127.0.0.1:8091",
				Endpoint:      "127.0.0.1:8091",
				AllowInsecure: false,
				TLS:           TLS{Enabled: false},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected cluster grpc insecure config to be rejected")
	}
	if !strings.Contains(err.Error(), "cluster.grpc.tls.enabled must be true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAllowsClusterInsecureGRPCWhenExplicitlyEnabled(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "/tmp/skytree",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:          "127.0.0.1:8091",
				Endpoint:      "127.0.0.1:8091",
				AllowInsecure: true,
				TLS:           TLS{Enabled: false},
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected explicit insecure cluster grpc config to be accepted: %v", err)
	}
}

func TestValidateRejectsWSSListenerWithoutBrokerTLS(t *testing.T) {
	cfg := AppConfig{
		Broker: Broker{
			Listen:             []string{"wss://127.0.0.1:8443"},
			TLS:                TLS{Enabled: false},
			ConnectAckProperty: validConnectAckPropertyForValidate(),
			MessageRetry:       DefaultMessageRetryConfig(),
		},
		Storage: validDeliveryStoreForValidate(),
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for wss listener without broker tls")
	}
	if !strings.Contains(err.Error(), "tls:// or wss://") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAcceptsWSSListenerWithBrokerTLS(t *testing.T) {
	cfg := AppConfig{
		Broker: Broker{
			Listen: []string{"wss://127.0.0.1:8443"},
			TLS: TLS{
				Enabled:  true,
				CertFile: "/tmp/cert.pem",
				KeyFile:  "/tmp/key.pem",
			},
			ConnectAckProperty: validConnectAckPropertyForValidate(),
			MessageRetry:       DefaultMessageRetryConfig(),
		},
		Storage: validDeliveryStoreForValidate(),
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected wss listener with tls to pass validation: %v", err)
	}
}

func TestValidateRejectsUnsupportedStorageDriver(t *testing.T) {
	cfg := AppConfig{
		Broker: validBrokerForValidate(),
		Storage: Store{
			Default: "unknown_driver",
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected unsupported storage.driver to fail validation")
	}
	if !strings.Contains(err.Error(), "unsupported storage.driver") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsUnsupportedBrokerListenerProtocol(t *testing.T) {
	cfg := AppConfig{
		Broker: Broker{
			Listen:             []string{"mqtt://127.0.0.1:1883"},
			ConnectAckProperty: validConnectAckPropertyForValidate(),
			MessageRetry:       DefaultMessageRetryConfig(),
		},
		Storage: validDeliveryStoreForValidate(),
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported broker listener protocol")
	}
	if !strings.Contains(err.Error(), "unsupported broker listener protocol") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBetaRejectsAllowInsecureClusterGRPC(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "/tmp/skytree",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:          "127.0.0.1:8091",
				Endpoint:      "127.0.0.1:8091",
				AllowInsecure: true,
			},
		},
	}

	err := ValidateForProfile(cfg, ValidationProfileBeta)
	if err == nil {
		t.Fatal("expected beta profile to reject allow_insecure")
	}
	if !strings.Contains(err.Error(), "cluster.grpc.allow_insecure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBetaAcceptsSecureClusterGRPC(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "/tmp/skytree",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:     "127.0.0.1:8091",
				Endpoint: "127.0.0.1:8091",
				TLS: TLS{
					Enabled:  true,
					CertFile: "/tmp/cert.pem",
					KeyFile:  "/tmp/key.pem",
				},
			},
		},
	}

	if err := ValidateForProfile(cfg, ValidationProfileBeta); err != nil {
		t.Fatalf("expected secure beta cluster config to pass: %v", err)
	}
}

func TestValidateRejectsClusterWithEmptyDataDir(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:          "127.0.0.1:8091",
				Endpoint:      "127.0.0.1:8091",
				AllowInsecure: true,
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected cluster config with empty data_dir to fail")
	}
	if !strings.Contains(err.Error(), "cluster.data_dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsBrokerOptionalMTLSWithoutCA(t *testing.T) {
	cfg := AppConfig{
		Broker: Broker{
			Listen: []string{"tls://127.0.0.1:8883"},
			TLS: TLS{
				Enabled:      true,
				CertFile:     "/tmp/cert.pem",
				KeyFile:      "/tmp/key.pem",
				MTLSAuthMode: "optional",
			},
			ConnectAckProperty: validConnectAckPropertyForValidate(),
			MessageRetry:       DefaultMessageRetryConfig(),
		},
		Storage: validDeliveryStoreForValidate(),
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected optional broker mTLS without ca_file to fail")
	}
	if !strings.Contains(err.Error(), "broker.tls.ca_file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsClusterGRPCOptionalMTLSWithoutCA(t *testing.T) {
	cfg := AppConfig{
		Broker:  validBrokerForValidate(),
		Storage: validScyllaDeliveryStoreForValidate(),
		Cluster: Cluster{
			Enable:           true,
			Join:             false,
			DataDir:          "/tmp/skytree",
			LocalNodeID:      1,
			LocalNodeAddress: "127.0.0.1:8080",
			Member: map[uint64]string{
				1: "127.0.0.1:8080",
			},
			GRPC: GRPC{
				Addr:     "127.0.0.1:8091",
				Endpoint: "127.0.0.1:8091",
				TLS: TLS{
					Enabled:      true,
					CertFile:     "/tmp/cert.pem",
					KeyFile:      "/tmp/key.pem",
					MTLSAuthMode: "optional",
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected cluster grpc optional mTLS without ca_file to fail")
	}
	if !strings.Contains(err.Error(), "cluster.grpc.tls.ca_file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
