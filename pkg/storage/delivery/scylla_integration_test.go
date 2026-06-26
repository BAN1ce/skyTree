//go:build integration && scylla

package delivery_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/storage/delivery/factory"
	"github.com/google/uuid"
)

func TestScyllaBackedDeliveryStoreEndToEnd(t *testing.T) {
	if os.Getenv("SKYTREE_SCYLLA_INTEGRATION") != "1" {
		t.Skip("set SKYTREE_SCYLLA_INTEGRATION=1 to run Scylla integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := factory.BuildClientDeliveryStoreWithContext(ctx, scyllaIntegrationConfig(), nil)
	if err != nil {
		t.Fatalf("BuildClientDeliveryStoreWithContext error: %v", err)
	}
	defer closeIfPossible(store)
	combinedStore, ok := store.(brokerstore.CombinedSubscriptionStore)
	if !ok {
		t.Fatalf("store type %T does not implement CombinedSubscriptionStore", store)
	}

	if err := store.EnsureDeliverySchema(ctx); err != nil {
		t.Fatalf("EnsureDeliverySchema error: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	clientID := "scylla-e2e-" + uuid.NewString()
	shareGroup := "scylla-share-" + uuid.NewString()
	topicFilter := "$share/" + shareGroup + "/jobs/+/created"
	messageID := uuid.New()
	taskID := uuid.New()
	payload := []byte("scylla-delivery-e2e-payload")

	if err := store.SaveMessagePayload(ctx, brokerstore.MessagePayloadRecord{
		CreatedAt:         now,
		MessageID:         messageID,
		PublishTopic:      "jobs/a/created",
		PublisherClientID: "scylla-e2e-publisher",
		Payload:           payload,
	}); err != nil {
		t.Fatalf("SaveMessagePayload error: %v", err)
	}
	gotPayload, err := store.LoadMessagePayload(ctx, messageID)
	if err != nil {
		t.Fatalf("LoadMessagePayload error: %v", err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatal("loaded payload does not match saved payload")
	}

	inserted, err := store.AppendDeliveryTask(ctx, brokerstore.DeliveryTask{
		TS:                now,
		TaskID:            taskID,
		ClientID:          clientID,
		MessageID:         messageID,
		DeliveryQoS:       1,
		SubscriptionIDs:   []int32{7},
		NoLocal:           true,
		RetainAsPublished: true,
	})
	if err != nil {
		t.Fatalf("AppendDeliveryTask error: %v", err)
	}
	if !inserted {
		t.Fatal("AppendDeliveryTask inserted=false, want true")
	}

	duplicateInserted, err := store.AppendDeliveryTask(ctx, brokerstore.DeliveryTask{
		TS:          now.Add(time.Millisecond),
		TaskID:      uuid.New(),
		ClientID:    clientID,
		MessageID:   messageID,
		DeliveryQoS: 1,
	})
	if err != nil {
		t.Fatalf("duplicate AppendDeliveryTask error: %v", err)
	}
	if duplicateInserted {
		t.Fatal("duplicate AppendDeliveryTask inserted=true, want false")
	}

	exists, err := store.DeliveryTaskExists(ctx, clientID, messageID)
	if err != nil {
		t.Fatalf("DeliveryTaskExists error: %v", err)
	}
	if !exists {
		t.Fatal("DeliveryTaskExists=false, want true")
	}

	tasks, err := store.ReadDeliveryTasks(ctx, clientID, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("ReadDeliveryTasks error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("ReadDeliveryTasks len=%d, want 1", len(tasks))
	}
	if tasks[0].TaskID != taskID || tasks[0].MessageID != messageID {
		t.Fatalf("unexpected delivery task: %+v", tasks[0])
	}

	advanced, err := store.AdvanceDeliveryCursor(ctx, brokerstore.DeliveryCursor{
		UpdatedTS:  now.Add(time.Second),
		ClientID:   clientID,
		Generation: tasks[0].Generation,
		LastTS:     tasks[0].TS,
		LastTaskID: tasks[0].TaskID,
	})
	if err != nil {
		t.Fatalf("AdvanceDeliveryCursor error: %v", err)
	}
	if !advanced {
		t.Fatal("AdvanceDeliveryCursor advanced=false, want true")
	}
	cursor, err := store.ReadDeliveryCursor(ctx, clientID)
	if err != nil {
		t.Fatalf("ReadDeliveryCursor error: %v", err)
	}
	if cursor == nil || cursor.LastTaskID != taskID {
		t.Fatalf("unexpected delivery cursor: %+v", cursor)
	}

	sharedTask := &sharedsubscription.ShareGroupTask{
		TaskID:          uuid.New(),
		ShareGroup:      shareGroup,
		TopicFilter:     topicFilter,
		MessageID:       uuid.New(),
		DeliveryQoS:     1,
		PublishQoS:      1,
		PublisherClient: "scylla-e2e-publisher",
		SubscriptionIDs: `[11]`,
		WinnerNoLocal:   true,
		WinnerRAP:       true,
		Status:          sharedsubscription.TaskStatusPending,
		Timestamp:       now,
	}
	if err := combinedStore.AppendShareGroupTask(ctx, now, sharedTask); err != nil {
		t.Fatalf("AppendShareGroupTask error: %v", err)
	}
	if err := expectSharedTaskStatus(ctx, combinedStore, shareGroup, sharedTask.TaskID, sharedsubscription.TaskStatusPending); err != nil {
		t.Fatal(err)
	}

	updated, err := combinedStore.AtomicUpdateTaskStatus(
		ctx,
		sharedTask.TaskID,
		shareGroup,
		sharedsubscription.TaskStatusPending,
		sharedsubscription.TaskStatusProcessing,
	)
	if err != nil {
		t.Fatalf("AtomicUpdateTaskStatus processing error: %v", err)
	}
	if !updated {
		t.Fatal("AtomicUpdateTaskStatus processing updated=false, want true")
	}

	processing, err := combinedStore.QueryProcessingTasksBefore(ctx, shareGroup, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("QueryProcessingTasksBefore error: %v", err)
	}
	if !hasSharedTask(processing, sharedTask.TaskID) {
		t.Fatalf("processing tasks do not include task %s", sharedTask.TaskID)
	}

	if err := combinedStore.RollbackSharedSubscriptionTask(ctx, sharedTask); err != nil {
		t.Fatalf("RollbackSharedSubscriptionTask error: %v", err)
	}
	if err := expectSharedTaskStatus(ctx, combinedStore, shareGroup, sharedTask.TaskID, sharedsubscription.TaskStatusPending); err != nil {
		t.Fatal(err)
	}

	byMessage, err := combinedStore.QueryShareGroupTaskByMessageID(
		ctx,
		shareGroup,
		sharedTask.MessageID,
		[]sharedsubscription.TaskStatus{sharedsubscription.TaskStatusPending},
	)
	if err != nil {
		t.Fatalf("QueryShareGroupTaskByMessageID error: %v", err)
	}
	if byMessage == nil || byMessage.TaskID != sharedTask.TaskID {
		t.Fatalf("unexpected task by message: %+v", byMessage)
	}
}

func scyllaIntegrationConfig() config.AppConfig {
	host := getenvDefault("SKYTREE_SCYLLA_HOST", "127.0.0.1")
	port, _ := strconv.Atoi(getenvDefault("SKYTREE_SCYLLA_PORT", "9042"))
	return config.AppConfig{
		Storage: config.Store{
			Default:        config.KeyStoreTypeBadger,
			MessageExpired: 1,
			DeliveryQueue: config.DeliveryQueueConfig{
				Type:           config.ClientDeliveryQueueTypeScylla,
				BucketDuration: time.Minute,
			},
			Payload: config.PayloadConfig{
				Type: config.ClientDeliveryPayloadTypeScylla,
			},
			Cassandra: config.Cassandra{
				Hosts:           splitHosts(host),
				Port:            port,
				Keyspace:        getenvDefault("SKYTREE_SCYLLA_KEYSPACE", "skytree"),
				Consistency:     getenvDefault("SKYTREE_SCYLLA_CONSISTENCY", "LOCAL_QUORUM"),
				Timeout:         10 * time.Second,
				ConnectTimeout:  10 * time.Second,
				NumConns:        2,
				AutoCreateTable: true,
			},
		},
		Cluster: config.Cluster{Enable: true, LocalNodeID: 1},
	}
}

func expectSharedTaskStatus(
	ctx context.Context,
	store brokerstore.CombinedSubscriptionStore,
	shareGroup string,
	taskID uuid.UUID,
	status sharedsubscription.TaskStatus,
) error {
	tasks, readErr := store.ReadShareGroupTasks(ctx, shareGroup, time.Time{}, uuid.Nil, 20)
	if readErr != nil {
		return readErr
	}
	for _, task := range tasks {
		if task != nil && task.TaskID == taskID && task.Status == status {
			return nil
		}
	}
	return brokerstore.ErrNotFound
}

func hasSharedTask(tasks []*sharedsubscription.ShareGroupTask, taskID uuid.UUID) bool {
	for _, task := range tasks {
		if task != nil && task.TaskID == taskID {
			return true
		}
	}
	return false
}

func getenvDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitHosts(value string) []string {
	parts := strings.Split(value, ",")
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		if host := strings.TrimSpace(part); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func closeIfPossible(v any) {
	closer, ok := v.(io.Closer)
	if ok && closer != nil {
		_ = closer.Close()
	}
}
