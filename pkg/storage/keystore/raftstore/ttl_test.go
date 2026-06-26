//go:build !race
// +build !race

package raftstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	badgerstore "github.com/BAN1ce/skyTree/pkg/storage/keystore/badger"
	badgerdb "github.com/dgraph-io/badger"
	"github.com/lni/dragonboat/v3/statemachine"
	"google.golang.org/protobuf/proto"
)

type fakeClusterClient struct {
	writeFn func(ctx context.Context, data []byte) (statemachine.Result, error)
}

func (f *fakeClusterClient) Write(ctx context.Context, data []byte) (statemachine.Result, error) {
	if f.writeFn == nil {
		return statemachine.Result{}, nil
	}
	return f.writeFn(ctx, data)
}

func (f *fakeClusterClient) Read(ctx context.Context, query interface{}) (interface{}, error) {
	return nil, nil
}

func (f *fakeClusterClient) GetNodeID() uint64 { return 1 }

func TestKeyStoreClusterSetExpiredEncodesTTLRequest(t *testing.T) {
	var gotReq *dbpb.Request
	cli := &fakeClusterClient{
		writeFn: func(ctx context.Context, data []byte) (statemachine.Result, error) {
			req := new(dbpb.Request)
			if err := proto.Unmarshal(data, req); err != nil {
				return statemachine.Result{}, err
			}
			gotReq = req
			return statemachine.Result{}, nil
		},
	}
	s := NewKeyStoreCluster(cli)

	wantTTL := 2 * time.Second
	if err := s.SetExpired(context.Background(), []byte("k"), wantTTL); err != nil {
		t.Fatalf("SetExpired() error = %v", err)
	}

	if gotReq == nil {
		t.Fatal("SetExpired() did not write raft request")
	}
	if gotReq.GetType() != dbpb.DB_REQUEST_TYPE_EXPIRE {
		t.Fatalf("request type = %v, want %v", gotReq.GetType(), dbpb.DB_REQUEST_TYPE_EXPIRE)
	}
	if len(gotReq.GetCMD()) != 1 || !bytes.Equal(gotReq.GetCMD()[0], []byte("k")) {
		t.Fatalf("request cmd = %v, want [[107]]", gotReq.GetCMD())
	}
	if gotReq.GetTtlNanos() != wantTTL.Nanoseconds() {
		t.Fatalf("request ttl_nanos = %d, want %d", gotReq.GetTtlNanos(), wantTTL.Nanoseconds())
	}
}

func TestKeyStoreClusterSetExpiredRejectsNonPositiveDuration(t *testing.T) {
	s := NewKeyStoreCluster(&fakeClusterClient{})
	if err := s.SetExpired(context.Background(), []byte("k"), 0); err == nil {
		t.Fatal("SetExpired() expected error for duration=0, got nil")
	}
}

func newTestBadgerStore(t *testing.T) *badgerstore.Badger {
	t.Helper()
	opt := badgerdb.DefaultOptions(filepath.Join(t.TempDir(), "db"))
	opt.Logger = nil
	s, err := badgerstore.NewBadger(opt)
	if err != nil {
		t.Fatalf("NewBadger() error = %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

func TestStateHandlerExpireKeepsValueAndExpires(t *testing.T) {
	db := newTestBadgerStore(t)
	h := NewStateHandler(db)

	ctx := context.Background()
	if err := db.PutKey(ctx, []byte("k"), []byte("v1")); err != nil {
		t.Fatalf("PutKey() error = %v", err)
	}

	req := &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      [][]byte{[]byte("k")},
		TtlNanos: (2 * time.Second).Nanoseconds(),
	}
	if _, err := h.HandleWrite(ctx, req); err != nil {
		t.Fatalf("HandleWrite(EXPIRE) error = %v", err)
	}

	val, ok, err := db.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey() error = %v", err)
	}
	if !ok || string(val) != "v1" {
		t.Fatalf("ReadKey() = (%q,%v), want (%q,true)", string(val), ok, "v1")
	}

	time.Sleep(3 * time.Second)
	_, ok, err = db.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey(after ttl) error = %v", err)
	}
	if ok {
		t.Fatalf("ReadKey(after ttl) ok = true, want false")
	}

	missingReq := &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      [][]byte{[]byte("missing")},
		TtlNanos: (2 * time.Second).Nanoseconds(),
	}
	if _, err := h.HandleWrite(ctx, missingReq); err != nil {
		t.Fatalf("HandleWrite(EXPIRE missing key) error = %v", err)
	}
}

type noExpireStore struct{}

func (n *noExpireStore) PutKey(context.Context, []byte, []byte) error { return nil }
func (n *noExpireStore) DeleteKey(context.Context, []byte) error      { return nil }
func (n *noExpireStore) ReadKey(context.Context, []byte) ([]byte, bool, error) {
	return nil, false, nil
}
func (n *noExpireStore) DeletePrefixKey(context.Context, []byte) error { return nil }
func (n *noExpireStore) HSet(context.Context, []byte, [][]byte) error  { return nil }
func (n *noExpireStore) HGet(context.Context, []byte, []byte) ([]byte, bool, error) {
	return nil, false, nil
}
func (n *noExpireStore) HDel(context.Context, []byte, [][]byte) error               { return nil }
func (n *noExpireStore) HGetAll(context.Context, []byte) (map[string]string, error) { return nil, nil }
func (n *noExpireStore) HPrefix(context.Context, []byte, []byte) (map[string]string, error) {
	return nil, nil
}
func (n *noExpireStore) DeleteHash(context.Context, []byte) error { return nil }
func (n *noExpireStore) Close() error                             { return nil }
func (n *noExpireStore) Snapshot(io.Writer) error                 { return nil }
func (n *noExpireStore) Recover(io.Reader) error                  { return nil }

var _ store.KeyStoreWithBackup = (*noExpireStore)(nil)

func TestStateHandlerExpireRequiresExpirer(t *testing.T) {
	h := NewStateHandler(&noExpireStore{})
	req := &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      [][]byte{[]byte("k")},
		TtlNanos: (1 * time.Second).Nanoseconds(),
	}
	_, err := h.HandleWrite(context.Background(), req)
	if err == nil {
		t.Fatal("HandleWrite(EXPIRE) expected error for non-expirer store, got nil")
	}
	if got, want := err.Error(), "does not support expiration"; !bytes.Contains([]byte(got), []byte(want)) {
		t.Fatalf("HandleWrite(EXPIRE) error = %q, want contains %q", got, want)
	}
}

func TestStateHandlerExpireRejectsInvalidRequest(t *testing.T) {
	h := NewStateHandler(newTestBadgerStore(t))
	_, err := h.HandleWrite(context.Background(), &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      nil,
		TtlNanos: (1 * time.Second).Nanoseconds(),
	})
	if err == nil || err.Error() != "invalid EXPIRE request: missing key" {
		t.Fatalf("HandleWrite(EXPIRE missing key) error = %v", err)
	}

	_, err = h.HandleWrite(context.Background(), &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      [][]byte{[]byte("k")},
		TtlNanos: 0,
	})
	if err == nil {
		t.Fatal("HandleWrite(EXPIRE ttl=0) expected error, got nil")
	}
	if got, want := err.Error(), "ttl_nanos must be > 0"; !bytes.Contains([]byte(got), []byte(want)) {
		t.Fatalf("HandleWrite(EXPIRE ttl=0) error = %q, want contains %q", got, want)
	}
}

func TestStateHandlerUnknownRequestTypeStillErrors(t *testing.T) {
	h := NewStateHandler(newTestBadgerStore(t))
	_, err := h.HandleWrite(context.Background(), &dbpb.Request{
		Type: dbpb.DB_REQUEST_TYPE(9999),
	})
	if err == nil {
		t.Fatal("HandleWrite(unknown) expected error, got nil")
	}
	if got, want := err.Error(), "invalid request type"; got != want {
		t.Fatalf("HandleWrite(unknown) error = %q, want %q", got, want)
	}
}

func TestNoExpireStoreImplementsKeyStoreWithBackup(t *testing.T) {
	var _ store.KeyStoreWithBackup = (*noExpireStore)(nil)
	if err := (&noExpireStore{}).Snapshot(io.Discard); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if err := (&noExpireStore{}).Recover(bytes.NewReader(nil)); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if err := (&noExpireStore{}).Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func ExampleStateHandler_HandleWrite_expireWithoutSupport() {
	h := NewStateHandler(&noExpireStore{})
	_, err := h.HandleWrite(context.Background(), &dbpb.Request{
		Type:     dbpb.DB_REQUEST_TYPE_EXPIRE,
		CMD:      [][]byte{[]byte("k")},
		TtlNanos: int64(time.Second),
	})
	fmt.Println(err != nil)
	// Output: true
}
