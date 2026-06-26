package badgerstore

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger"
)

var defaultBadgerUpdateRetryBackoff = []time.Duration{
	2 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
}

func withBadgerUpdateRetry(update func() error) error {
	return withBadgerUpdateRetryConfig(update, defaultBadgerUpdateRetryBackoff, time.Sleep)
}

func withBadgerUpdateRetryConfig(
	update func() error,
	backoff []time.Duration,
	sleep func(time.Duration),
) error {
	for attempt := 0; ; attempt++ {
		err := update()
		if !isBadgerUpdateRetryable(err) {
			return err
		}
		if attempt >= len(backoff) {
			return err
		}
		sleep(backoff[attempt])
	}
}

func isBadgerUpdateRetryable(err error) bool {
	return errors.Is(err, badger.ErrConflict) || errors.Is(err, badger.ErrRetry)
}
