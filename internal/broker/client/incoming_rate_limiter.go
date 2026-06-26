package client

import (
	"sync"
	"time"
)

type incomingPublishRateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	windowStart time.Time
	count       int
}

func newIncomingPublishRateLimiter(limit int, window time.Duration) *incomingPublishRateLimiter {
	if limit <= 0 {
		return nil
	}
	if window <= 0 {
		window = time.Second
	}
	return &incomingPublishRateLimiter{
		limit:  limit,
		window: window,
	}
}

func (l *incomingPublishRateLimiter) allow(now time.Time) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= l.window {
		l.windowStart = now
		l.count = 0
	}
	if l.count >= l.limit {
		return false
	}
	l.count++
	return true
}
