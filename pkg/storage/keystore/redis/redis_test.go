package redis

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/config"
)

func TestSetExpiredRejectsNonPositiveDuration(t *testing.T) {
	store := NewRedis(config.Redis{Address: "127.0.0.1:6379"})
	t.Cleanup(func() {
		_ = store.Close()
	})

	if err := store.SetExpired(context.Background(), []byte("k"), 0); err == nil {
		t.Fatal("SetExpired() expected error for duration=0, got nil")
	}
}
