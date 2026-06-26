package qos2receive

import (
	"sync"
	"testing"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestNewQoS2ReceiveStore(t *testing.T) {
	store := NewQoS2ReceiveStore()
	if store == nil {
		t.Error("NewQoS2ReceiveStore() returned nil")
	}
	if store.waiting == nil {
		t.Error("NewQoS2ReceiveStore() waiting map is nil")
	}
	if len(store.waiting) != 0 {
		t.Error("NewQoS2ReceiveStore() waiting map should be empty")
	}
}

func TestQoS2ReceiveStore_Store(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Normal store.
	publish := &brokerpublish.Message{
		Publish: &packets.Publish{
			PacketID: 1,
		},
	}

	// The first store should return false (not existed).
	existed := store.Store(publish)
	if existed {
		t.Error("First store should return false (not existed)")
	}

	// The second store should return true (existed).
	existed = store.Store(publish)
	if !existed {
		t.Error("Second store should return true (existed)")
	}

	// Verify the message is stored.
	message, ok := store.Read(1)
	if !ok {
		t.Error("Message should be found after store")
	}
	if message != publish {
		t.Error("Stored message should be the same as input")
	}
}

func TestQoS2ReceiveStore_Store_NilMessage(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Store nil message.
	existed := store.Store(nil)
	if existed {
		t.Error("Store nil message should return false")
	}

	// Verify no message is stored.
	if len(store.waiting) != 0 {
		t.Error("No message should be stored when input is nil")
	}
}

func TestQoS2ReceiveStore_Store_NilPublishPacket(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Store message with nil Publish.
	message := &brokerpublish.Message{
		Publish: nil,
	}

	existed := store.Store(message)
	if existed {
		t.Error("Store message with nil Publish should return false")
	}

	// Verify no message is stored.
	if len(store.waiting) != 0 {
		t.Error("No message should be stored when Publish is nil")
	}
}

func TestQoS2ReceiveStore_Read(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Read non-existent message.
	message, ok := store.Read(999)
	if ok {
		t.Error("Read non-existent message should return false")
	}
	if message != nil {
		t.Error("Read non-existent message should return nil")
	}

	// Store then read.
	publish := &brokerpublish.Message{
		Publish: &packets.Publish{
			PacketID: 123,
		},
	}
	store.Store(publish)

	message, ok = store.Read(123)
	if !ok {
		t.Error("Read existing message should return true")
	}
	if message != publish {
		t.Error("Read should return the correct message")
	}
}

func TestQoS2ReceiveStore_ReadDoesNotAutoExpireWaitingPubrel(t *testing.T) {
	store := NewQoS2ReceiveStore()
	message := &brokerpublish.Message{
		Publish: &packets.Publish{
			PacketID: 321,
		},
	}
	store.Store(message)

	store.mux.Lock()
	store.lastSeen[321] = time.Now().Add(-defaultQoS2WaitingTTL - time.Second)
	store.mux.Unlock()

	got, ok := store.Read(321)
	if !ok {
		t.Fatal("read must not auto-expire QoS2 state while the session is active")
	}
	if got != message {
		t.Fatalf("expected original message, got %p", got)
	}
}

func TestQoS2ReceiveStore_Delete(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Delete non-existent message.
	publishPacket, ok := store.Delete(999)
	if ok {
		t.Error("Delete non-existent message should return false")
	}
	if publishPacket != nil {
		t.Error("Delete non-existent message should return nil")
	}

	// Store then delete.
	message := &brokerpublish.Message{
		Publish: &packets.Publish{
			PacketID: 456,
		},
	}
	store.Store(message)

	deletedMessage, ok := store.Delete(456)
	if !ok {
		t.Error("Delete existing message should return true")
	}
	if deletedMessage != message {
		t.Error("Delete should return the correct message")
	}

	// Verify the message is deleted.
	_, ok = store.Read(456)
	if ok {
		t.Error("Message should be deleted after Delete call")
	}
}

func TestQoS2ReceiveStore_ConcurrentAccess(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Concurrent write test.
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrently store different messages.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id uint16) {
			defer wg.Done()
			message := &brokerpublish.Message{
				Publish: &packets.Publish{
					PacketID: id,
				},
			}
			store.Store(message)
		}(uint16(i))
	}

	wg.Wait()

	// Verify all messages are stored.
	if len(store.waiting) != numGoroutines {
		t.Errorf("Expected %d messages, got %d", numGoroutines, len(store.waiting))
	}

	// Concurrent read test.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id uint16) {
			defer wg.Done()
			_, ok := store.Read(id)
			if !ok {
				t.Errorf("Message %d should exist", id)
			}
		}(uint16(i))
	}

	wg.Wait()

	// Concurrent delete test.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id uint16) {
			defer wg.Done()
			_, ok := store.Delete(id)
			if !ok {
				t.Errorf("Delete message %d should succeed", id)
			}
		}(uint16(i))
	}

	wg.Wait()

	// Verify all messages are deleted.
	if len(store.waiting) != 0 {
		t.Errorf("Expected 0 messages after delete, got %d", len(store.waiting))
	}
}

func TestQoS2ReceiveStore_StoreReadDeleteFlow(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Full store-read-delete flow.
	message := &brokerpublish.Message{
		Publish: &packets.Publish{
			PacketID: 789,
		},
	}

	// 1) Store.
	existed := store.Store(message)
	if existed {
		t.Error("First store should return false")
	}

	// 2) Read.
	retrievedMessage, ok := store.Read(789)
	if !ok {
		t.Error("Read should succeed after store")
	}
	if retrievedMessage != message {
		t.Error("Retrieved message should be the same as stored")
	}

	// 3) Delete.
	deletedMessage, ok := store.Delete(789)
	if !ok {
		t.Error("Delete should succeed")
	}
	if deletedMessage != message {
		t.Error("Deleted message should be the same as stored")
	}

	// 4) Verify it cannot be read after delete.
	_, ok = store.Read(789)
	if ok {
		t.Error("Read should fail after delete")
	}
}

func TestQoS2ReceiveStore_MultipleMessages(t *testing.T) {
	store := NewQoS2ReceiveStore()

	// Store multiple messages.
	messages := make([]*brokerpublish.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = &brokerpublish.Message{
			Publish: &packets.Publish{
				PacketID: uint16(i + 1),
			},
		}
		store.Store(messages[i])
	}

	// Verify all messages are stored.
	if len(store.waiting) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(store.waiting))
	}

	// Verify all messages can be read.
	for i := 0; i < 10; i++ {
		message, ok := store.Read(uint16(i + 1))
		if !ok {
			t.Errorf("Message %d should exist", i+1)
		}
		if message != messages[i] {
			t.Errorf("Message %d should be the same as stored", i+1)
		}
	}

	// Delete some messages.
	for i := 0; i < 5; i++ {
		_, ok := store.Delete(uint16(i + 1))
		if !ok {
			t.Errorf("Delete message %d should succeed", i+1)
		}
	}

	// Verify remaining messages.
	if len(store.waiting) != 5 {
		t.Errorf("Expected 5 messages after partial delete, got %d", len(store.waiting))
	}

	// Verify remaining messages can be read.
	for i := 5; i < 10; i++ {
		_, ok := store.Read(uint16(i + 1))
		if !ok {
			t.Errorf("Message %d should still exist", i+1)
		}
	}
}

func TestQoS2ReceiveStore_CleanupExpired(t *testing.T) {
	store := NewQoS2ReceiveStore()
	now := time.Now()

	// Insert two entries and manually adjust timestamps.
	m1 := &brokerpublish.Message{Publish: &packets.Publish{PacketID: 1}}
	m2 := &brokerpublish.Message{Publish: &packets.Publish{PacketID: 2}}
	store.Store(m1)
	store.Store(m2)

	// Force one entry to be expired.
	store.mux.Lock()
	store.lastSeen[1] = now.Add(-2 * time.Minute)
	store.lastSeen[2] = now.Add(-10 * time.Second)
	store.mux.Unlock()

	removed := store.CleanupExpired(30*time.Second, now)
	if removed != 1 {
		t.Fatalf("expected removed=1, got %d", removed)
	}
	if _, ok := store.Read(1); ok {
		t.Fatalf("expected packetID=1 to be expired and removed")
	}
	if _, ok := store.Read(2); !ok {
		t.Fatalf("expected packetID=2 to remain")
	}
}

func TestQoS2ReceiveStore_CountAutoCleanupExpired(t *testing.T) {
	store := NewQoS2ReceiveStore()
	store.Store(&brokerpublish.Message{Publish: &packets.Publish{PacketID: 7}})

	store.mux.Lock()
	store.lastSeen[7] = time.Now().Add(-11 * time.Minute)
	store.mux.Unlock()

	if got := store.Count(); got != 0 {
		t.Fatalf("expected store count to be 0 after auto cleanup, got %d", got)
	}
	if _, ok := store.Read(7); ok {
		t.Fatalf("expected expired packetID=7 to be removed by count cleanup")
	}
}

func TestQoS2ReceiveStore_StoreAutoCleanupExpired(t *testing.T) {
	store := NewQoS2ReceiveStore()
	store.Store(&brokerpublish.Message{Publish: &packets.Publish{PacketID: 1}})

	store.mux.Lock()
	store.lastSeen[1] = time.Now().Add(-11 * time.Minute)
	store.mux.Unlock()

	store.Store(&brokerpublish.Message{Publish: &packets.Publish{PacketID: 2}})

	if _, ok := store.Read(1); ok {
		t.Fatalf("expected expired packetID=1 to be removed during store")
	}
	if _, ok := store.Read(2); !ok {
		t.Fatalf("expected packetID=2 to exist")
	}
}
