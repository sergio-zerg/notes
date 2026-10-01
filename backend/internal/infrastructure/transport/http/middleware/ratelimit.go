package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"notes/internal/ratelimit"
)

// RateLimiter throttles requests per client address using a token bucket.
//
// Buckets are keyed by client IP so that one noisy caller cannot consume
// another's budget. Each bucket refills at rps tokens per second up to a burst
// allowance, which absorbs short spikes while rejecting sustained load.
//
// The bucket mechanics live in internal/ratelimit because the gRPC transport
// enforces the same budget; this type contributes only the client key derived
// from the request.
type RateLimiter struct {
	limiter *ratelimit.Limiter
}

// NewRateLimiter creates a limiter allowing rps requests per second with the
// given burst allowance per client.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{limiter: ratelimit.New(rps, burst)}
}

// RateLimitMiddleware rejects callers that exceed their budget with 429.
func (l *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.limiter.Allow(clientKey(r)) {
			// The exhausted duration tells a well-behaved client when to
			// retry instead of hammering immediately. It is whole seconds,
			// which is the only form the Retry-After header can carry.
			if retry := l.limiter.RetryAfter(); retry > 0 {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(retry.Seconds()), 10))
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// StartCleanup runs the bucket sweep in the background and returns a function
// that stops it. Buckets for clients that have genuinely gone away hold memory
// otherwise, and a long-lived process behind many NATs would otherwise
// accumulate one entry per observed address.
func (l *RateLimiter) StartCleanup(ctx context.Context, every, ttl time.Duration) func() {
	return l.limiter.StartCleanup(ctx, every, ttl)
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
