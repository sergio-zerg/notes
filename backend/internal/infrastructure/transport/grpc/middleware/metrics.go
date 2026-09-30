package middleware

import (
	"context"
	"time"

	"notes/internal/infrastructure/observability"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// GrpcMetricsInterceptor records Prometheus metrics and duration for gRPC calls.
func GrpcMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		st, _ := status.FromError(err)
		code := st.Code().String()
		method := info.FullMethod

		observability.GrpcRequestsTotal.WithLabelValues(method, code).Inc()
		observability.GrpcRequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())

		return resp, err
	}
}
