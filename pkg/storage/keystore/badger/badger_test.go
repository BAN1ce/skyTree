//go:build !race
// +build !race

package badger

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger"
)

func newTestBadger(t *testing.T) *Badger {
	t.Helper()

	opt := badgerdb.DefaultOptions(filepath.Join(t.TempDir(), "db"))
	opt.Logger = nil
	store, err := NewBadger(opt)
	if err != nil {
		t.Fatalf("NewBadger() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestReadKeyReturnsCopiedValueAndExistence(t *testing.T) {
	store := newTestBadger(t)
	ctx := context.Background()

	if err := store.PutKey(ctx, []byte("k"), []byte("v1")); err != nil {
		t.Fatalf("PutKey() error = %v", err)
	}

	got, ok, err := store.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("ReadKey() ok = false, want true")
	}
	if string(got) != "v1" {
		t.Fatalf("ReadKey() value = %q, want %q", string(got), "v1")
	}

	got[0] = 'x'
	got2, ok, err := store.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey() 2 error = %v", err)
	}
	if !ok {
		t.Fatalf("ReadKey() 2 ok = false, want true")
	}
	if string(got2) != "v1" {
		t.Fatalf("ReadKey() 2 value = %q, want %q", string(got2), "v1")
	}
}

func TestSetExpiredKeepsValueAndSupportsMissingKey(t *testing.T) {
	store := newTestBadger(t)
	ctx := context.Background()

	if err := store.PutKey(ctx, []byte("k"), []byte("v1")); err != nil {
		t.Fatalf("PutKey() error = %v", err)
	}

	if err := store.SetExpired(ctx, []byte("k"), 2*time.Second); err != nil {
		t.Fatalf("SetExpired(existing) error = %v", err)
	}
	got, ok, err := store.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey(after SetExpired) error = %v", err)
	}
	if !ok || string(got) != "v1" {
		t.Fatalf("ReadKey(after SetExpired) = (%q,%v), want (%q,true)", string(got), ok, "v1")
	}

	time.Sleep(3 * time.Second)
	_, ok, err = store.ReadKey(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("ReadKey(after ttl) error = %v", err)
	}
	if ok {
		t.Fatalf("ReadKey(after ttl) ok = true, want false")
	}

	if err := store.SetExpired(ctx, []byte("missing"), 2*time.Second); err != nil {
		t.Fatalf("SetExpired(missing) error = %v", err)
	}
}

func TestSetExpiredRejectsNonPositiveDuration(t *testing.T) {
	store := newTestBadger(t)
	ctx := context.Background()
	if err := store.SetExpired(ctx, []byte("k"), 0); err == nil {
		t.Fatal("SetExpired() expected error for duration=0, got nil")
	}
}

func TestHSetRejectsOddFieldWithoutPartialWrite(t *testing.T) {
	store := newTestBadger(t)
	ctx := context.Background()
	key := []byte("hash:")

	err := store.HSet(ctx, key, [][]byte{
		[]byte("f1"), []byte("v1"), []byte("orphan"),
	})
	if err == nil {
		t.Fatal("HSet() expected error for odd field length, got nil")
	}

	if _, ok, getErr := store.HGet(ctx, key, []byte("f1")); getErr != nil {
		t.Fatalf("HGet() error = %v", getErr)
	} else if ok {
		t.Fatal("HGet() found partial write for f1, want not found")
	}
}

func TestHSetAcceptsEvenAndEmptyField(t *testing.T) {
	store := newTestBadger(t)
	ctx := context.Background()
	key := []byte("hash:")

	if err := store.HSet(ctx, key, [][]byte{[]byte("f1"), []byte("v1")}); err != nil {
		t.Fatalf("HSet(even) error = %v", err)
	}

	got, ok, err := store.HGet(ctx, key, []byte("f1"))
	if err != nil {
		t.Fatalf("HGet() error = %v", err)
	}
	if !ok || string(got) != "v1" {
		t.Fatalf("HGet() = (%q, %v), want (%q, true)", string(got), ok, "v1")
	}

	if err := store.HSet(ctx, key, nil); err != nil {
		t.Fatalf("HSet(empty) error = %v", err)
	}
}
