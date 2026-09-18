package middleware

import (
	"fmt"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type RateLimiter struct {
	mu sync.Mutex

	buckets map[string]*tokenBucket

	tokensPerSecond float64
	burst           float64
	inactiveAfter   time.Duration
	cleanupInterval time.Duration

	now func() time.Time

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func NewRateLimiter(
	requests int,
	interval time.Duration,
	burst int,
	inactiveAfter time.Duration,
	cleanupInterval time.Duration,
) (*RateLimiter, error) {
	return newRateLimiter(
		requests,
		interval,
		burst,
		inactiveAfter,
		cleanupInterval,
		time.Now,
	)
}

func newRateLimiter(
	requests int,
	interval time.Duration,
	burst int,
	inactiveAfter time.Duration,
	cleanupInterval time.Duration,
	now func() time.Time,
) (*RateLimiter, error) {
	switch {
	case requests <= 0:
		return nil, fmt.Errorf(
			"rate limiter requests must be greater than zero",
		)

	case interval <= 0:
		return nil, fmt.Errorf(
			"rate limiter interval must be greater than zero",
		)

	case burst <= 0:
		return nil, fmt.Errorf(
			"rate limiter burst must be greater than zero",
		)

	case inactiveAfter <= 0:
		return nil, fmt.Errorf(
			"rate limiter inactivity duration must be greater than zero",
		)

	case cleanupInterval <= 0:
		return nil, fmt.Errorf(
			"rate limiter cleanup interval must be greater than zero",
		)

	case now == nil:
		return nil, fmt.Errorf(
			"rate limiter clock cannot be nil",
		)
	}

	limiter := &RateLimiter{
		buckets:         make(map[string]*tokenBucket),
		tokensPerSecond: float64(requests) / interval.Seconds(),
		burst:           float64(burst),
		inactiveAfter:   inactiveAfter,
		cleanupInterval: cleanupInterval,
		now:             now,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}

	go limiter.runCleanup()

	return limiter, nil
}

func (l *RateLimiter) Allow(key string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     l.burst,
			lastRefill: now,
			lastSeen:   now,
		}

		l.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * l.tokensPerSecond

		if bucket.tokens > l.burst {
			bucket.tokens = l.burst
		}

		bucket.lastRefill = now
	}

	bucket.lastSeen = now

	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--

	return true
}
func (l *RateLimiter) runCleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()
	defer close(l.doneCh)

	for {
		select {
		case <-ticker.C:
			l.removeStale(l.now())

		case <-l.stopCh:
			return
		}
	}
}

func (l *RateLimiter) removeStale(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) >= l.inactiveAfter {
			delete(l.buckets, key)
		}
	}
}

func (l *RateLimiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})

	<-l.doneCh
}
