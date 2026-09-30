package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter throttles requests per client address using a token bucket.
//
// Buckets are keyed by client IP so that one noisy caller cannot consume
// another's budget. Each bucket refills at rps tokens per second up to a burst
// allowance, which absorbs short spikes while rejecting sustained load.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	rps     rate.Limit
	burst   int

	// lastSeen records when a bucket was last used so idle clients can be
	// evicted. Without it the map grows with every distinct address seen.
	lastSeen map[string]time.Time
}

// NewRateLimiter creates a limiter allowing rps requests per second with the
// given burst allowance per client.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		buckets:  make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		lastSeen: make(map[string]time.Time),
	}
}

// allow consumes one token for the client identified by r.
func (l *RateLimiter) allow(r *http.Request) bool {
	key := clientKey(r)

	l.mu.Lock()
	limiter, ok := l.buckets[key]
	if !ok {
		limiter = rate.NewLimiter(l.rps, l.burst)
		l.buckets[key] = limiter
	}
	l.lastSeen[key] = time.Now()
	l.mu.Unlock()

	// reserve() then Allow() would double-consume; Allow() alone is the
	// correct single call for a plain permit check.
	return limiter.Allow()
}

// RateLimitMiddleware rejects callers that exceed their budget with 429.
func (l *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(r) {
			// The exhausted duration tells a well-behaved client when to
			// retry instead of hammering immediately.
			if retry := l.retryAfter(r); retry > 0 {
				w.Header().Set("Retry-After", retry.String())
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// retryAfter reports how long a caller must wait before one token refills.
// With a bucket refilling at rps, a full bucket of n costs about (n-1)/rps to
// refill, so that is the wait worth advertising.
func (l *RateLimiter) retryAfter(r *http.Request) time.Duration {
	seconds := float64(l.burst-1) / float64(l.rps)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// StartCleanup runs the bucket sweep in the background and returns a function
// that stops it. Buckets for clients that have genuinely gone away hold memory
// otherwise, and a long-lived process behind many NATs would otherwise
// accumulate one entry per observed address.
func (l *RateLimiter) StartCleanup(ctx context.Context, every, ttl time.Duration) func() {
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
func (l *RateLimiter) cleanup(ttl time.Duration) {
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

// clientKey identifies the caller. The remote address is used, ignoring the
// port; X-Forwarded-For is deliberately not trusted because it is client
// controlled and would let a caller mint unlimited buckets, or evade one.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
