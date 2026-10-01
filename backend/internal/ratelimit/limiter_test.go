package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A bucket of n permits exactly n requests before rejecting.
func TestAllowRespectsBurst(t *testing.T) {
	l := New(1, 2)

	for i := range 2 {
		if !l.Allow("client") {
			t.Fatalf("request %d was rejected, want allowed", i+1)
		}
	}
	if l.Allow("client") {
		t.Error("request 3 was allowed, want rejected")
	}
}

// One client's spending must not consume another's budget.
func TestAllowSeparatesKeys(t *testing.T) {
	l := New(1, 1)

	if !l.Allow("first") {
		t.Fatal("first client was rejected, want allowed")
	}
	if !l.Allow("second") {
		t.Error("second client shared the first client's budget")
	}
	if l.Allow("first") {
		t.Error("first client was not charged for its own request")
	}
}

// Retry-After is a whole number of seconds, and must describe the wait for one
// token rather than a full bucket refill. Advertising the refill time made a
// client at rps=20, burst=40 wait two seconds for a budget that refills every
// 50ms.
func TestRetryAfterIsOneTokenRoundedUp(t *testing.T) {
	cases := []struct {
		rps   float64
		burst int
		want  time.Duration
	}{
		{20, 40, time.Second},     // 1/20s rounds up to 1s
		{1, 3, time.Second},       // exactly 1s
		{0.5, 1, 2 * time.Second}, // 1/0.5 = 2s
		{0.1, 1, 10 * time.Second},
	}

	for _, tc := range cases {
		if got := New(tc.rps, tc.burst).RetryAfter(); got != tc.want {
			t.Errorf("New(%v, %d).RetryAfter() = %v, want %v", tc.rps, tc.burst, got, tc.want)
		}
	}
}

// A degenerate configuration must not produce a limiter that rejects every
// call or one that never refills.
func TestNewClampsDegenerateSettings(t *testing.T) {
	l := New(0, 0)

	if !l.Allow("client") {
		t.Error("a clamped limiter rejected the very first request")
	}
	if got := l.RetryAfter(); got != time.Second {
		t.Errorf("RetryAfter() = %v, want 1s", got)
	}
}

// Buckets for clients that have gone away must be evicted, or a process behind
// many NATs leaks one entry per observed key.
func TestCleanupEvictsIdleBuckets(t *testing.T) {
	l := New(10, 10)
	l.Allow("idle")

	if got := l.bucketCount(); got != 1 {
		t.Fatalf("bucketCount() = %d, want 1", got)
	}

	// A ttl in the past evicts the bucket that was just used.
	l.cleanup(-time.Second)

	if got := l.bucketCount(); got != 0 {
		t.Errorf("bucketCount() after sweep = %d, want 0", got)
	}
}

// A bucket still inside its ttl must survive a sweep.
func TestCleanupKeepsActiveBuckets(t *testing.T) {
	l := New(10, 10)
	l.Allow("active")

	l.cleanup(time.Hour)

	if got := l.bucketCount(); got != 1 {
		t.Errorf("bucketCount() = %d, want 1 for a bucket inside its ttl", got)
	}
}

func TestStartCleanupStops(t *testing.T) {
	l := New(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stop := l.StartCleanup(ctx, time.Millisecond, time.Millisecond)

	time.Sleep(5 * time.Millisecond)
	stop()
	cancel()
}

// Cancelling the context must also stop the sweep, or the goroutine outlives
// the shutdown signal.
func TestStartCleanupStopsOnContextCancel(t *testing.T) {
	l := New(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stop := l.StartCleanup(ctx, time.Millisecond, time.Millisecond)
	defer stop()

	cancel()
	time.Sleep(5 * time.Millisecond)

	// The sweep goroutine has exited, so a bucket created afterwards is still
	// the only one present.
	l.Allow("after")
	if got := l.bucketCount(); got != 1 {
		t.Errorf("bucketCount() = %d, want 1", got)
	}
}

// Concurrent clients must not race on the bucket map. The counter is atomic
// because the race detector would otherwise flag the test itself.
func TestAllowIsConcurrencySafe(t *testing.T) {
	l := New(1000, 1000)

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if l.Allow(string(rune('a' + i%26))) {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if got := allowed.Load(); got != 20 {
		t.Errorf("allowed = %d, want 20", got)
	}
}
