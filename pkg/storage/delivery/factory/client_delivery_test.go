package factory_test

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/storage/delivery/factory"
	payloadbadger "github.com/BAN1ce/skyTree/pkg/storage/delivery/messagepayload/badger"
	"github.com/google/uuid"
)

func TestResolveClientDeliverySpec_RequiresExplicitDrivers(t *testing.T) {
	cfg := config.AppConfig{
		Storage: config.Store{
			Default: config.KeyStoreTypeBadger,
		},
		Cluster: config.Cluster{Enable: false},
	}
	if _, err := cfg.ResolveClientDeliverySpec(); err == nil {
		t.Fatal("expected ResolveClientDeliverySpec to fail when drivers are missing")
	}
}

func TestResolveClientDeliverySpec_RejectsLegacyOrMixedDrivers(t *testing.T) {
	tests := []struct {
		name        string
		queueType   string
		payloadType string
	}{
		{
			name:        "legacy queue local_badger is rejected",
			queueType:   "local_badger",
			payloadType: config.ClientDeliveryPayloadTypeSingleNodeBadger,
		},
		{
			name:        "legacy queue raft is rejected",
			queueType:   "raft",
			payloadType: config.ClientDeliveryPayloadTypeScylla,
		},
		{
			name:        "legacy payload redis is rejected",
			queueType:   config.ClientDeliveryQueueTypeScylla,
			payloadType: "redis",
		},
		{
			name:        "mixed pair is rejected",
			queueType:   config.ClientDeliveryQueueTypeSingleNodeBadger,
			payloadType: config.ClientDeliveryPayloadTypeScylla,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.AppConfig{
				Storage: config.Store{
					Default: config.KeyStoreTypeBadger,
					DeliveryQueue: config.DeliveryQueueConfig{
						Type: tt.queueType,
					},
					Payload: config.PayloadConfig{
						Type: tt.payloadType,
					},
				},
				Cluster: config.Cluster{Enable: false},
			}
			if _, err := cfg.ResolveClientDeliverySpec(); err == nil {
				t.Fatal("expected ResolveClientDeliverySpec to fail")
			}
		})
	}
}

func TestResolveClientDeliverySpec_AllowsOnlyStrictPairs(t *testing.T) {
	tests := []struct {
		name           string
		queueType      string
		payloadType    string
		clusterEnabled bool
	}{
		{
			name:           "single-node badger pair in standalone mode",
			queueType:      config.ClientDeliveryQueueTypeSingleNodeBadger,
			payloadType:    config.ClientDeliveryPayloadTypeSingleNodeBadger,
			clusterEnabled: false,
		},
		{
			name:           "scylla pair in standalone mode",
			queueType:      config.ClientDeliveryQueueTypeScylla,
			payloadType:    config.ClientDeliveryPayloadTypeScylla,
			clusterEnabled: false,
		},
		{
			name:           "scylla pair in cluster mode",
			queueType:      config.ClientDeliveryQueueTypeScylla,
			payloadType:    config.ClientDeliveryPayloadTypeScylla,
			clusterEnabled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.AppConfig{
				Storage: config.Store{
					Default: config.KeyStoreTypeBadger,
					DeliveryQueue: config.DeliveryQueueConfig{
						Type: tt.queueType,
					},
					Payload: config.PayloadConfig{
						Type: tt.payloadType,
					},
				},
				Cluster: config.Cluster{Enable: tt.clusterEnabled},
			}
			spec, err := cfg.ResolveClientDeliverySpec()
			if err != nil {
				t.Fatalf("ResolveClientDeliverySpec error: %v", err)
			}
			if spec.QueueType != tt.queueType || spec.PayloadType != tt.payloadType {
				t.Fatalf("unexpected spec: %+v", spec)
			}
		})
	}
}

func TestBuildDeliveryMetadataStore_SingleNodeBadgerWorks(t *testing.T) {
	if raceEnabled {
		t.Skip("skip local badger integration under -race (badger checkptr incompatibility)")
	}
	tmp := t.TempDir()
	cfg := config.AppConfig{
		Storage: config.Store{
			Default: config.KeyStoreTypeRedis,
			Badger:  config.Badger{Path: tmp},
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: config.Cluster{Enable: false, LocalNodeID: 1},
	}
	s, err := factory.BuildDeliveryMetadataStore(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.EnsureDeliverySchema(ctx); err != nil {
		t.Fatalf("EnsureDeliverySchema error: %v", err)
	}
}

func TestBuildMessagePayloadStore_ScyllaAliasReusesCQLPath(t *testing.T) {
	cfg := config.AppConfig{
		Storage: config.Store{
			Default: config.KeyStoreTypeRedis,
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeScylla,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeScylla,
			},
			// Keep empty to force constructor-level validation without external dependency.
			Cassandra: config.Cassandra{},
		},
		Cluster: config.Cluster{Enable: true},
	}

	_, err := factory.BuildMessagePayloadStore(cfg, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "cassandra hosts is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildClientDeliveryStore_SingleNodeBadgerRoundTrip(t *testing.T) {
	if raceEnabled {
		t.Skip("skip local badger integration under -race (badger checkptr incompatibility)")
	}
	tmp := t.TempDir()
	cfg := config.AppConfig{
		Storage: config.Store{
			Default: config.KeyStoreTypeBadger,
			Badger:  config.Badger{Path: tmp},
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: config.Cluster{Enable: false, LocalNodeID: 42},
	}

	s, err := factory.BuildClientDeliveryStore(cfg, nil)
	if err != nil {
		t.Fatalf("BuildClientDeliveryStore error: %v", err)
	}
	defer closeIfPossible(s)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.EnsureDeliverySchema(ctx); err != nil {
		t.Fatalf("EnsureDeliverySchema error: %v", err)
	}

	now := time.Now().UTC()
	messageID := uuid.New()
	taskID := uuid.New()
	wantPayload := []byte("hello-delivery-payload")
	if err := s.SaveMessagePayload(ctx, brokerstore.MessagePayloadRecord{
		CreatedAt: now,
		MessageID: messageID,
		Payload:   wantPayload,
	}); err != nil {
		t.Fatalf("SaveMessagePayload error: %v", err)
	}

	inserted, err := s.AppendDeliveryTask(ctx, brokerstore.DeliveryTask{
		TS:          now,
		TaskID:      taskID,
		ClientID:    "client-a",
		MessageID:   messageID,
		DeliveryQoS: 1,
	})
	if err != nil {
		t.Fatalf("AppendDeliveryTask error: %v", err)
	}
	if !inserted {
		t.Fatalf("AppendDeliveryTask inserted=false, want true")
	}

	tasks, err := s.ReadDeliveryTasks(ctx, "client-a", time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadDeliveryTasks error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("ReadDeliveryTasks len=%d, want 1", len(tasks))
	}
	if tasks[0].TaskID != taskID || tasks[0].MessageID != messageID {
		t.Fatalf("unexpected task: %+v", tasks[0])
	}

	gotPayload, err := s.LoadMessagePayload(ctx, messageID)
	if err != nil {
		t.Fatalf("LoadMessagePayload error: %v", err)
	}
	if !bytes.Equal(gotPayload, wantPayload) {
		t.Fatalf("payload mismatch: got=%q want=%q", string(gotPayload), string(wantPayload))
	}
}

func closeIfPossible(v any) {
	closer, ok := v.(io.Closer)
	if ok && closer != nil {
		_ = closer.Close()
	}
}

func TestBuildMessagePayloadStore_MessageExpireDaysUsesDayUnit(t *testing.T) {
	if raceEnabled {
		t.Skip("skip local badger integration under -race (badger checkptr incompatibility)")
	}
	cfg := config.AppConfig{
		Storage: config.Store{
			MessageExpired: 1,
			Badger: config.Badger{
				Path: t.TempDir(),
			},
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: config.Cluster{Enable: false, LocalNodeID: 7},
	}

	store, err := factory.BuildMessagePayloadStore(cfg, nil)
	if err != nil {
		t.Fatalf("BuildMessagePayloadStore error: %v", err)
	}
	closeIfPossible(store)

	badgerStore, ok := store.(*payloadbadger.MessagePayloadStore)
	if !ok {
		t.Fatalf("unexpected payload store type: %T", store)
	}

	ttlField := reflect.ValueOf(badgerStore).Elem().FieldByName("ttl")
	if !ttlField.IsValid() {
		t.Fatal("ttl field not found")
	}
	gotTTL := time.Duration(ttlField.Int())
	if gotTTL != 24*time.Hour {
		t.Fatalf("ttl = %v, want %v", gotTTL, 24*time.Hour)
	}
}

func TestBuildMessagePayloadStore_ExtendsTTLToSessionExpiryLimit(t *testing.T) {
	if raceEnabled {
		t.Skip("skip local badger integration under -race (badger checkptr incompatibility)")
	}
	cfg := config.AppConfig{
		Broker: config.Broker{
			Limits: config.BrokerLimits{
				SessionExpiryMaxSeconds: uint32((7 * 24 * time.Hour) / time.Second),
			},
		},
		Storage: config.Store{
			MessageExpired: 1,
			Badger: config.Badger{
				Path: t.TempDir(),
			},
			DeliveryQueue: config.DeliveryQueueConfig{
				Type: config.ClientDeliveryQueueTypeSingleNodeBadger,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeSingleNodeBadger,
			},
		},
		Cluster: config.Cluster{Enable: false, LocalNodeID: 7},
	}

	store, err := factory.BuildMessagePayloadStore(cfg, nil)
	if err != nil {
		t.Fatalf("BuildMessagePayloadStore error: %v", err)
	}
	closeIfPossible(store)

	badgerStore, ok := store.(*payloadbadger.MessagePayloadStore)
	if !ok {
		t.Fatalf("unexpected payload store type: %T", store)
	}
	ttlField := reflect.ValueOf(badgerStore).Elem().FieldByName("ttl")
	if !ttlField.IsValid() {
		t.Fatal("ttl field not found")
	}
	gotTTL := time.Duration(ttlField.Int())
	if gotTTL != 7*24*time.Hour {
		t.Fatalf("ttl = %v, want %v", gotTTL, 7*24*time.Hour)
	}
}
