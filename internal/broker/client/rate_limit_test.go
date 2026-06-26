package client

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
)

func TestBucket_Unlimited_ReturnsImmediately(t *testing.T) {
	b := rate.NewBucket(0)
	if b == nil {
		t.Fatalf("bucket is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	b.GetToken(ctx)
	if time.Since(start) > 10*time.Millisecond {
		t.Fatalf("unlimited bucket should not block")
	}
}

func TestBucket_Limited_BlocksUntilContextDoneWhenEmpty(t *testing.T) {
	b := rate.NewBucket(1)
	if b == nil {
		t.Fatalf("bucket is nil")
	}

	// Consume the only token.
	b.GetToken(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	b.GetToken(ctx)
	elapsed := time.Since(start)
	if elapsed < 20*time.Millisecond {
		t.Fatalf("expected bucket to block until ctx done, elapsed=%v", elapsed)
	}
}

func TestBucket_PutTokenRestoresAvailability(t *testing.T) {
	b := rate.NewBucket(1)
	b.GetToken(context.Background()) // drain
	b.PutToken()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	b.GetToken(ctx)
	if time.Since(start) > 10*time.Millisecond {
		t.Fatalf("expected GetToken to return quickly after PutToken")
	}
}
