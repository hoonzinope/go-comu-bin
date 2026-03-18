package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.RateLimiter = (*InMemoryRateLimiter)(nil)

type InMemoryRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateLimitBucket
}

type rateLimitBucket struct {
	count   int
	resetAt time.Time
}

func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		buckets: make(map[string]rateLimitBucket),
	}
}

func (l *InMemoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	_ = ctx
	if key == "" || limit <= 0 || window <= 0 {
		return true, nil
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	for existingKey, existingBucket := range l.buckets {
		if !existingBucket.resetAt.After(now) {
			delete(l.buckets, existingKey)
		}
	}

	bucket, ok := l.buckets[key]
	if !ok || !bucket.resetAt.After(now) {
		l.buckets[key] = rateLimitBucket{
			count:   1,
			resetAt: now.Add(window),
		}
		return true, nil
	}
	if bucket.count >= limit {
		return false, nil
	}
	bucket.count++
	l.buckets[key] = bucket
	return true, nil
}
