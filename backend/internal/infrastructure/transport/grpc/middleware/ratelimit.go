package middleware

import (
	"context"
	"net"
	"strconv"
	"time"

	"notes/internal/ratelimit"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// GrpcRateLimiter throttles RPCs per client address, mirroring the HTTP
// limiter so that both transports enforce the same budget.
//
// The bucket mechanics live in internal/ratelimit; this type contributes only
// the client key derived from the call context.
type GrpcRateLimiter struct {
	limiter *ratelimit.Limiter
}

// NewGrpcRateLimiter creates a per-client limiter for the gRPC transport.
func NewGrpcRateLimiter(rps float64, burst int) *GrpcRateLimiter {
	return &GrpcRateLimiter{limiter: ratelimit.New(rps, burst)}
}

// RetryAfterHeader carries the wait in whole seconds, matching the HTTP
// limiter's Retry-After so a client reading either transport looks in the same
// place.
const RetryAfterHeader = "retry-after"

// RateLimitUnaryInterceptor rejects RPCs that exceed the caller's budget.
func (l *GrpcRateLimiter) RateLimitUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if !l.limiter.Allow(grpcClientKey(ctx)) {
			// The wait travels as trailer metadata rather than inside the
			// message: a client backoff reads a field, and parsing the delay out
			// of prose meant every generated client had to know the wording of
			// this sentence to throttle correctly.
			retry := l.limiter.RetryAfter()
			// A failure here only means the metadata will be absent, which
			// leaves the status code as the signal — not worth losing the
			// rejection over.
			_ = grpc.SetTrailer(ctx, metadata.Pairs(
				RetryAfterHeader, strconv.FormatInt(int64(retry.Seconds()), 10),
			))

			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// StartCleanup sweeps buckets idle for longer than ttl until ctx is cancelled
// or the returned function is called. Without it the map grows with every
// distinct peer address ever observed.
func (l *GrpcRateLimiter) StartCleanup(ctx context.Context, every, ttl time.Duration) func() {
	return l.limiter.StartCleanup(ctx, every, ttl)
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
