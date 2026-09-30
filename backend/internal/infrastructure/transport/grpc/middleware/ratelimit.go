package middleware

import (
	"context"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// GrpcRateLimiter throttles RPCs per client address, mirroring the HTTP
// limiter so that both transports enforce the same budget.
type GrpcRateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*rate.Limiter
	lastSeen map[string]time.Time
	rps      rate.Limit
	burst    int
}

// NewGrpcRateLimiter creates a per-client limiter for the gRPC transport.
func NewGrpcRateLimiter(rps float64, burst int) *GrpcRateLimiter {
	if rps <= 0 {
		rps = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &GrpcRateLimiter{
		buckets:  make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// RateLimitUnaryInterceptor rejects RPCs that exceed the caller's budget.
func (l *GrpcRateLimiter) RateLimitUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if !l.allow(ctx) {
			return nil, status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded, retry after about %.1fs", l.retryAfterSeconds())
		}
		return handler(ctx, req)
	}
}

// retryAfterSeconds is how long a bucket of n tokens takes to refill, which is
// the wait a well-behaved client should observe before retrying.
func (l *GrpcRateLimiter) retryAfterSeconds() float64 {
	return float64(l.burst-1) / float64(l.rps)
}

func (l *GrpcRateLimiter) allow(ctx context.Context) bool {
	key := grpcClientKey(ctx)

	l.mu.Lock()
	limiter, ok := l.buckets[key]
	if !ok {
		limiter = rate.NewLimiter(l.rps, l.burst)
		l.buckets[key] = limiter
	}
	l.lastSeen[key] = time.Now()
	l.mu.Unlock()

	return limiter.Allow()
}

// StartCleanup sweeps buckets idle for longer than ttl until ctx is cancelled.
// Without it the map grows with every distinct peer address ever observed.
func (l *GrpcRateLimiter) StartCleanup(ctx context.Context, every, ttl time.Duration) {
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.sweep(ttl)
			}
		}
	}()
}

func (l *GrpcRateLimiter) sweep(ttl time.Duration) {
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

// grpcClientKey identifies the caller by peer address. Proxy headers are
// deliberately ignored: they are client controlled and would let a caller mint
// unlimited buckets, or evade an existing one.
func grpcClientKey(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}
