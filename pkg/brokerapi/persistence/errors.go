package persistence

import (
	"context"
	"errors"
	"net"

	"github.com/dgraph-io/badger"
	"github.com/gocql/gocql"
	"github.com/nutsdb/nutsdb"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrNotFound means requested persistence data does not exist.
	ErrNotFound = errors.New("persistence: not found")
	// ErrRetryable means operation failure may succeed on retry.
	ErrRetryable = errors.New("persistence: retryable")
	// ErrConflict means optimistic concurrency/update conflict.
	ErrConflict = errors.New("persistence: conflict")
	// ErrUnsupported means requested capability is not implemented by backend.
	ErrUnsupported = errors.New("persistence: unsupported")
)

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrMessagePayloadNotFound) ||
		errors.Is(err, redis.Nil) ||
		errors.Is(err, nutsdb.ErrBucket) ||
		errors.Is(err, badger.ErrKeyNotFound) ||
		errors.Is(err, gocql.ErrNotFound)
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRetryable) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func IsConflict(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrConflict)
}

func IsUnsupported(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrUnsupported)
}
