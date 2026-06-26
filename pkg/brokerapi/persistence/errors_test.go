package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/dgraph-io/badger"
	"github.com/gocql/gocql"
	"github.com/nutsdb/nutsdb"
	"github.com/redis/go-redis/v9"
)

type retryableNetErr struct{}

func (retryableNetErr) Error() string   { return "retryable" }
func (retryableNetErr) Timeout() bool   { return true }
func (retryableNetErr) Temporary() bool { return true }

func TestIsNotFoundMappings(t *testing.T) {
	notFoundErrors := []error{
		ErrNotFound,
		ErrMessagePayloadNotFound,
		redis.Nil,
		nutsdb.ErrBucket,
		badger.ErrKeyNotFound,
		gocql.ErrNotFound,
	}

	for _, err := range notFoundErrors {
		if !IsNotFound(err) {
			t.Fatalf("expected IsNotFound=true for %v", err)
		}
	}

	if IsNotFound(errors.New("other")) {
		t.Fatal("expected IsNotFound=false for unrelated error")
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(ErrRetryable) {
		t.Fatal("expected ErrRetryable to be retryable")
	}
	if !IsRetryable(context.DeadlineExceeded) {
		t.Fatal("expected context deadline to be retryable")
	}
	if !IsRetryable(retryableNetErr{}) {
		t.Fatal("expected temporary net error to be retryable")
	}
	if IsRetryable(errors.New("permanent")) {
		t.Fatal("expected permanent error to be non-retryable")
	}
}

func TestIsConflict(t *testing.T) {
	if !IsConflict(ErrConflict) {
		t.Fatal("expected ErrConflict to be conflict")
	}
	if IsConflict(errors.New("other")) {
		t.Fatal("expected unrelated error to be non-conflict")
	}
}
