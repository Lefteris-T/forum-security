package middleware

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu      sync.Mutex
	current time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{
		current: time.Date(
			2026,
			time.January,
			1,
			0,
			0,
			0,
			0,
			time.UTC,
		),
	}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.current
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.current = c.current.Add(duration)
}
func TestRateLimiterAllowsBurstThenRejects(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		5,
		time.Minute,
		3,
		10*time.Minute,
		time.Minute,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}
	t.Cleanup(limiter.Stop)

	for request := 1; request <= 3; request++ {
		if !limiter.Allow("client-a") {
			t.Fatalf(
				"request %d was rejected, want allowed",
				request,
			)
		}
	}

	if limiter.Allow("client-a") {
		t.Fatal("request after burst was allowed, want rejected")
	}
}

func TestRateLimiterRefillsUsingElapsedTime(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		6,
		time.Minute,
		2,
		10*time.Minute,
		time.Minute,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}
	t.Cleanup(limiter.Stop)

	if !limiter.Allow("client-a") {
		t.Fatal("first request was rejected")
	}

	if !limiter.Allow("client-a") {
		t.Fatal("second request was rejected")
	}

	if limiter.Allow("client-a") {
		t.Fatal("third request was allowed with empty bucket")
	}

	// Six tokens per minute means one new token every ten seconds.
	clock.Advance(10 * time.Second)

	if !limiter.Allow("client-a") {
		t.Fatal("request was rejected after one token refilled")
	}

	if limiter.Allow("client-a") {
		t.Fatal("extra request was allowed after only one token refilled")
	}
}

func TestRateLimiterKeepsClientsIndependent(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		1,
		time.Minute,
		1,
		10*time.Minute,
		time.Minute,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}
	t.Cleanup(limiter.Stop)

	if !limiter.Allow("client-a") {
		t.Fatal("client-a first request was rejected")
	}

	if limiter.Allow("client-a") {
		t.Fatal("client-a second request was allowed")
	}

	if !limiter.Allow("client-b") {
		t.Fatal("client-b was affected by client-a bucket")
	}
}
func TestRateLimiterRemovesOnlyStaleBuckets(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		1,
		time.Minute,
		1,
		5*time.Minute,
		time.Hour,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}

	t.Cleanup(limiter.Stop)

	limiter.Allow("stale-client")
	limiter.Allow("active-client")

	clock.Advance(4 * time.Minute)

	// Refresh only the active client's lastSeen value.
	limiter.Allow("active-client")

	clock.Advance(2 * time.Minute)

	limiter.removeStale(clock.Now())

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if _, exists := limiter.buckets["stale-client"]; exists {
		t.Error("stale client bucket was not removed")
	}

	if _, exists := limiter.buckets["active-client"]; !exists {
		t.Error("active client bucket was removed")
	}

	if len(limiter.buckets) != 1 {
		t.Errorf(
			"bucket count = %d, want 1",
			len(limiter.buckets),
		)
	}
}

func TestRateLimiterStopIsIdempotent(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		1,
		time.Minute,
		1,
		5*time.Minute,
		time.Hour,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}

	t.Cleanup(limiter.Stop)

	done := make(chan struct{})

	go func() {
		var waitGroup sync.WaitGroup

		for call := 0; call < 10; call++ {
			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()
				limiter.Stop()
			}()
		}

		waitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All Stop calls returned safely.

	case <-time.After(time.Second):
		t.Fatal("concurrent Stop calls did not return")
	}
}

func TestRateLimiterConcurrentAccessRespectsBurst(t *testing.T) {
	clock := newFakeClock()

	const (
		burst    = 50
		requests = 500
	)

	limiter, err := newRateLimiter(
		1,
		time.Minute,
		burst,
		5*time.Minute,
		time.Hour,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}

	t.Cleanup(limiter.Stop)

	var allowed atomic.Int64
	var waitGroup sync.WaitGroup

	for request := 0; request < requests; request++ {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			if limiter.Allow("shared-client") {
				allowed.Add(1)
			}
		}()
	}

	waitGroup.Wait()

	if got := allowed.Load(); got != burst {
		t.Errorf(
			"allowed requests = %d, want %d",
			got,
			burst,
		)
	}
}
func TestNewRateLimiterRejectsInvalidConfiguration(t *testing.T) {
	validClock := newFakeClock()

	tests := []struct {
		name            string
		requests        int
		interval        time.Duration
		burst           int
		inactiveAfter   time.Duration
		cleanupInterval time.Duration
		now             func() time.Time
	}{
		{
			name:            "zero requests",
			requests:        0,
			interval:        time.Minute,
			burst:           1,
			inactiveAfter:   time.Minute,
			cleanupInterval: time.Minute,
			now:             validClock.Now,
		},
		{
			name:            "zero interval",
			requests:        1,
			interval:        0,
			burst:           1,
			inactiveAfter:   time.Minute,
			cleanupInterval: time.Minute,
			now:             validClock.Now,
		},
		{
			name:            "zero burst",
			requests:        1,
			interval:        time.Minute,
			burst:           0,
			inactiveAfter:   time.Minute,
			cleanupInterval: time.Minute,
			now:             validClock.Now,
		},
		{
			name:            "zero inactivity duration",
			requests:        1,
			interval:        time.Minute,
			burst:           1,
			inactiveAfter:   0,
			cleanupInterval: time.Minute,
			now:             validClock.Now,
		},
		{
			name:            "zero cleanup interval",
			requests:        1,
			interval:        time.Minute,
			burst:           1,
			inactiveAfter:   time.Minute,
			cleanupInterval: 0,
			now:             validClock.Now,
		},
		{
			name:            "nil clock",
			requests:        1,
			interval:        time.Minute,
			burst:           1,
			inactiveAfter:   time.Minute,
			cleanupInterval: time.Minute,
			now:             nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := newRateLimiter(
				tt.requests,
				tt.interval,
				tt.burst,
				tt.inactiveAfter,
				tt.cleanupInterval,
				tt.now,
			)
			if err == nil {
				limiter.Stop()
				t.Fatal(
					"newRateLimiter() error = nil, want an error",
				)
			}
		})
	}
}

func TestRateLimiterRefillDoesNotExceedBurst(t *testing.T) {
	clock := newFakeClock()

	limiter, err := newRateLimiter(
		60,
		time.Minute,
		2,
		2*time.Hour,
		time.Hour,
		clock.Now,
	)
	if err != nil {
		t.Fatalf("newRateLimiter() error: %v", err)
	}

	t.Cleanup(limiter.Stop)

	limiter.Allow("client-a")
	limiter.Allow("client-a")

	clock.Advance(time.Hour)

	if !limiter.Allow("client-a") {
		t.Fatal("first request after refill was rejected")
	}

	if !limiter.Allow("client-a") {
		t.Fatal("second request after refill was rejected")
	}

	if limiter.Allow("client-a") {
		t.Fatal("refill exceeded burst capacity")
	}
}
