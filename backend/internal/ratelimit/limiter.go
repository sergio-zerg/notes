// Package ratelimit provides the per-client token bucket shared by the HTTP
// and gRPC transports.
//
// The two transports must enforce the same budget, and before this package
// each carried its own copy of the bucket map, the idle sweep and the retry
// arithmetic. The duplication was not harmless: a fix to the sweep reached one
// transport and left the other leaking buckets. The bucket mechanics live here
// and each transport contributes only the one thing that genuinely differs —
// how it derives a client key from a request.
package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter throttles requests per client key using a token bucket.
//
// Each bucket refills at rps tokens per second up to a burst allowance, which
// absorbs short spikes while rejecting sustained load. Buckets are keyed
// separately so that one noisy caller cannot consume another's budget.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	rps     rate.Limit
	burst   int

	// lastSeen records when a bucket was last used so idle clients can be
	// evicted. Without it the map grows with every distinct key ever seen.
	lastSeen map[string]time.Time
}

// New creates a limiter allowing rps requests per second with the given burst
// allowance per client.
//
// Non-positive rps and non-positive burst fall back to 1 rather than producing
// a limiter that rejects every call or panics: a bucket with a zero burst is
// never refilled, and a non-positive rate is not a meaningful budget.
func New(rps float64, burst int) *Limiter {
	if rps <= 0 {
		rps = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		buckets:  make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		lastSeen: make(map[string]time.Time),
	}
}

// Allow consumes one token for the given client key and reports whether the
// request may proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	limiter, ok := l.buckets[key]
	if !ok {
		limiter = rate.NewLimiter(l.rps, l.burst)
		l.buckets[key] = limiter
	}
	l.lastSeen[key] = time.Now()
	l.mu.Unlock()

	// reserve() followed by Allow() would double-consume; Allow() alone is the
	// correct single call for a plain permit check.
	return limiter.Allow()
}

// RetryAfter reports how long a caller must wait before its next request has a
// chance of succeeding.
//
// It is the time to refill a single token, 1/rps, rounded up to whole seconds
// because that is what the Retry-After header can express. The earlier
// implementation advertised a full bucket refill, (burst-1)/rps, which told a
// well-behaved client to wait up to two seconds for a budget that refills
// twenty times a second.
func (l *Limiter) RetryAfter() time.Duration {
	// l.rps is always positive, so the division cannot panic or yield infinity.
	seconds := int64(math.Ceil(1 / float64(l.rps)))
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

// StartCleanup runs the bucket sweep in the background and returns a function
// that stops it. Buckets for clients that have genuinely gone away hold memory
// otherwise, and a long-lived process behind many NATs would otherwise
// accumulate one entry per observed key.
func (l *Limiter) StartCleanup(ctx context.Context, every, ttl time.Duration) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				l.cleanup(ttl)
			}
		}
	}()
	return func() { close(done) }
}

// cleanup drops buckets untouched for longer than ttl.
func (l *Limiter) cleanup(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)

	l.mu.Lock()
	defer l.mu.Unlock()
	for key, seen := range l.lastSeen {
		if seen.Before(cutoff) {
			delete(l.buckets, key)
			delete(l.lastSeen, key)
		}
	}
}

// bucketCount reports how many buckets are live. It exists for the package's
// own tests, which assert on eviction; production code has no use for it.
func (l *Limiter) bucketCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
