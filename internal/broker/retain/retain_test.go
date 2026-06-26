package retain

import (
	"context"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/proto/proto_retain"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

func init() {
	// Initialize a minimal logger without depending on func() error { _, err := config.Load(""); return err }().
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
}

type memKeyStore struct {
	mu     sync.Mutex
	hashes map[string]map[string][]byte // key -> field -> value
}

func newMemKeyStore() *memKeyStore {
	return &memKeyStore{
		hashes: make(map[string]map[string][]byte),
	}
}

func (m *memKeyStore) PutKey(_ context.Context, _ []byte, _ []byte) error { return nil }

func (m *memKeyStore) ReadKey(_ context.Context, _ []byte) ([]byte, bool, error) {
	return nil, false, nil
}

func (m *memKeyStore) DeleteKey(_ context.Context, _ []byte) error { return nil }

func (m *memKeyStore) DeletePrefixKey(_ context.Context, _ []byte) error { return nil }

func (m *memKeyStore) HSet(_ context.Context, key []byte, field [][]byte) error {
	if len(field) < 2 {
		return nil
	}
	k := string(key)
	f := string(field[0])
	v := append([]byte(nil), field[1]...)

	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[k]
	if !ok {
		h = make(map[string][]byte)
		m.hashes[k] = h
	}
	h[f] = v
	return nil
}

func (m *memKeyStore) HGet(_ context.Context, key, field []byte) ([]byte, bool, error) {
	k := string(key)
	f := string(field)

	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[k]
	if !ok {
		return nil, false, nil
	}
	v, ok := h[f]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), v...), true, nil
}

func (m *memKeyStore) HDel(_ context.Context, key []byte, field [][]byte) error {
	k := string(key)

	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[k]
	if !ok {
		return nil
	}
	for _, f := range field {
		delete(h, string(f))
	}
	return nil
}

func (m *memKeyStore) HGetAll(_ context.Context, key []byte) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string)
	for field, value := range m.hashes[string(key)] {
		out[field] = string(value)
	}
	return out, nil
}

func (m *memKeyStore) HPrefix(_ context.Context, _ []byte, _ []byte) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *memKeyStore) DeleteHash(_ context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hashes, string(key))
	return nil
}

func (m *memKeyStore) Close() error { return nil }

func (m *memKeyStore) SetExpired(_ context.Context, _ []byte, _ time.Duration) error { return nil }

var _ brokerstore.HashStore = (*memKeyStore)(nil)

func TestNewRetainStore(t *testing.T) {
	retain := NewRetainStore(newMemKeyStore())
	if retain.store == nil {
		t.Errorf("expect non-nil store")
	}
}

func TestStore_PutRetainMessage(t *testing.T) {
	var (
		message = &proto_retain.RetainMessage{
			Topic:   "xxx",
			Payload: []byte("payload"),
			Qos:     1,
		}
	)

	// Sanity: message should marshal.
	if _, err := proto.Marshal(message); err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	retain := NewRetainStore(newMemKeyStore())

	if err := retain.PutRetainMessage(message); err != nil {
		t.Errorf("expect nil, but got %v", err)
	}

	retainMessage, ok := retain.GetRetainMessage(message.Topic)
	if !ok {
		t.Errorf("expect true, but got %v", ok)
	}

	if retainMessage.Topic != message.Topic {
		t.Errorf("expect %v, but got %v", message.Topic, retainMessage.Topic)
	}

	if !reflect.DeepEqual(retainMessage.Payload, message.Payload) {
		t.Errorf("expect %v, but got %v", message.Payload, retainMessage.Payload)
	}

	if retainMessage.Qos != message.Qos {
		t.Errorf("expect %v, but got %v", message.Qos, retainMessage.Qos)
	}

	if err := retain.DeleteRetainMessage(message.Topic); err != nil {
		t.Errorf("expect nil, but got %v", err)
	}

}

func TestStore_GetRetainMessagesByTopicFilter(t *testing.T) {
	retain := NewRetainStore(newMemKeyStore())
	messages := []*proto_retain.RetainMessage{
		{Topic: "sensor/1", Payload: []byte("one"), Qos: 1},
		{Topic: "sensor/2/temp", Payload: []byte("two"), Qos: 1},
		{Topic: "other/1", Payload: []byte("other"), Qos: 1},
	}
	for _, msg := range messages {
		if err := retain.PutRetainMessage(msg); err != nil {
			t.Fatalf("put retain %q: %v", msg.Topic, err)
		}
	}

	got, err := retain.GetRetainMessagesByTopicFilter("sensor/#")
	if err != nil {
		t.Fatalf("GetRetainMessagesByTopicFilter: %v", err)
	}
	gotTopics := make(map[string]bool, len(got))
	for _, msg := range got {
		gotTopics[msg.GetTopic()] = true
	}

	if !gotTopics["sensor/1"] || !gotTopics["sensor/2/temp"] {
		t.Fatalf("expected sensor retained topics, got %v", gotTopics)
	}
	if gotTopics["other/1"] {
		t.Fatalf("did not expect non-matching retained topic, got %v", gotTopics)
	}
}

func TestStore_RunGCOnceRemovesExpiredRetained(t *testing.T) {
	store := NewRetainStore(newMemKeyStore())
	now := time.Now()
	// 已过期：绝对过期时间在过去。
	expired := &proto_retain.RetainMessage{
		Topic:             "expired/topic",
		Payload:           []byte("old"),
		Qos:               0,
		ExpiredAtUnixNano: now.Add(-time.Second).UnixNano(),
	}
	// 未过期：绝对过期时间在未来。
	fresh := &proto_retain.RetainMessage{
		Topic:             "fresh/topic",
		Payload:           []byte("new"),
		Qos:               0,
		ExpiredAtUnixNano: now.Add(time.Hour).UnixNano(),
	}
	if err := store.PutRetainMessage(expired); err != nil {
		t.Fatalf("put expired: %v", err)
	}
	if err := store.PutRetainMessage(fresh); err != nil {
		t.Fatalf("put fresh: %v", err)
	}

	removed, err := store.RunGCOnce(context.Background())
	if err != nil {
		t.Fatalf("RunGCOnce: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 expired removal, got %d", removed)
	}
	if _, ok := store.GetRetainMessage("expired/topic"); ok {
		t.Fatal("expected expired retained to be gone")
	}
	if _, ok := store.GetRetainMessage("fresh/topic"); !ok {
		t.Fatal("expected fresh retained to remain")
	}
}

func TestStore_StartGCWithPredicateSkipsWhenFalse(t *testing.T) {
	keyStore := newMemKeyStore()
	store := NewRetainStore(keyStore)
	expired := &proto_retain.RetainMessage{
		Topic:             "predicate/skip",
		Payload:           []byte("old"),
		ExpiredAtUnixNano: time.Now().Add(-time.Second).UnixNano(),
	}
	if err := store.PutRetainMessage(expired); err != nil {
		t.Fatalf("put expired: %v", err)
	}

	store.StartGCWithPredicate(context.Background(), time.Millisecond, func(context.Context) bool {
		return false
	})
	time.Sleep(20 * time.Millisecond)
	store.StopGC()

	keyStore.mu.Lock()
	defer keyStore.mu.Unlock()
	if _, ok := keyStore.hashes[string(GetTopicRetainKey())]["predicate/skip"]; !ok {
		t.Fatal("expected retained message to remain when GC predicate is false")
	}
}

func TestStore_StartGCWithPredicateRunsWhenTrue(t *testing.T) {
	keyStore := newMemKeyStore()
	store := NewRetainStore(keyStore)
	expired := &proto_retain.RetainMessage{
		Topic:             "predicate/run",
		Payload:           []byte("old"),
		ExpiredAtUnixNano: time.Now().Add(-time.Second).UnixNano(),
	}
	if err := store.PutRetainMessage(expired); err != nil {
		t.Fatalf("put expired: %v", err)
	}

	store.StartGCWithPredicate(context.Background(), time.Millisecond, func(context.Context) bool {
		return true
	})
	defer store.StopGC()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		keyStore.mu.Lock()
		_, exists := keyStore.hashes[string(GetTopicRetainKey())]["predicate/run"]
		keyStore.mu.Unlock()
		if !exists {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("expected retained message to be removed when GC predicate is true")
}

func TestStore_GetRetainMessageLazyDeletesExpired(t *testing.T) {
	store := NewRetainStore(newMemKeyStore())
	expired := &proto_retain.RetainMessage{
		Topic:             "lazy/topic",
		Payload:           []byte("payload"),
		Qos:               0,
		ExpiredAtUnixNano: time.Now().Add(-time.Second).UnixNano(),
	}
	if err := store.PutRetainMessage(expired); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, ok := store.GetRetainMessage("lazy/topic"); ok {
		t.Fatal("expected lazy delete to drop expired retained on read")
	}

	// 第二次读取应继续返回 false（已经被惰性删除了）。
	if _, ok := store.GetRetainMessage("lazy/topic"); ok {
		t.Fatal("expired retained must remain absent after lazy delete")
	}
}

func TestStore_ExpiredAtUnixNanoZeroMeansNoExpiry(t *testing.T) {
	store := NewRetainStore(newMemKeyStore())
	msg := &proto_retain.RetainMessage{
		Topic:   "no-expiry/topic",
		Payload: []byte("payload"),
		Qos:     0,
	}
	if err := store.PutRetainMessage(msg); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, ok := store.GetRetainMessage("no-expiry/topic"); !ok {
		t.Fatal("expected retained message without ExpiredAtUnixNano to remain available")
	}
}
