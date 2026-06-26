package badgerstore

import (
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/badger"
)

func TestWithBadgerUpdateRetryRetriesRetryableErrors(t *testing.T) {
	tests := []struct {
		name        string
		failures    []error
		wantErr     error
		wantCalls   int
		wantSleeps  []time.Duration
		backoff     []time.Duration
		finalErr    error
		finalCalled bool
	}{
		{
			name:       "conflict retries until success",
			failures:   []error{badger.ErrConflict, badger.ErrConflict},
			wantCalls:  3,
			wantSleeps: []time.Duration{time.Millisecond, 2 * time.Millisecond},
			backoff:    []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond},
		},
		{
			name:       "badger retry retries until success",
			failures:   []error{badger.ErrRetry},
			wantCalls:  2,
			wantSleeps: []time.Duration{time.Millisecond},
			backoff:    []time.Duration{time.Millisecond},
		},
		{
			name:       "non retryable error returns immediately",
			failures:   []error{errors.New("permanent")},
			wantErr:    errors.New("permanent"),
			wantCalls:  1,
			wantSleeps: []time.Duration{},
			backoff:    []time.Duration{time.Millisecond},
		},
		{
			name:       "retryable error stops after backoff is exhausted",
			failures:   []error{badger.ErrConflict, badger.ErrConflict},
			wantErr:    badger.ErrConflict,
			wantCalls:  2,
			wantSleeps: []time.Duration{time.Millisecond},
			backoff:    []time.Duration{time.Millisecond},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			sleeps := []time.Duration{}
			err := withBadgerUpdateRetryConfig(
				func() error {
					calls++
					if calls <= len(tt.failures) {
						return tt.failures[calls-1]
					}
					return tt.finalErr
				},
				tt.backoff,
				func(d time.Duration) {
					sleeps = append(sleeps, d)
				},
			)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("withBadgerUpdateRetryConfig error = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
				t.Fatalf("withBadgerUpdateRetryConfig error = %v, want %v", err, tt.wantErr)
			}
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if len(sleeps) != len(tt.wantSleeps) {
				t.Fatalf("len(sleeps) = %d, want %d", len(sleeps), len(tt.wantSleeps))
			}
			for i, got := range sleeps {
				if got != tt.wantSleeps[i] {
					t.Fatalf("sleeps[%d] = %s, want %s", i, got, tt.wantSleeps[i])
				}
			}
		})
	}
}

func TestIsBadgerUpdateRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "conflict", err: badger.ErrConflict, want: true},
		{name: "retry", err: badger.ErrRetry, want: true},
		{name: "wrapped conflict", err: errors.Join(errors.New("append task"), badger.ErrConflict), want: true},
		{name: "permanent", err: errors.New("permanent"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBadgerUpdateRetryable(tt.err); got != tt.want {
				t.Fatalf("isBadgerUpdateRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
