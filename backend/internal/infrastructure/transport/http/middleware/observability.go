package middleware

import (
	"net/http"
	"strconv"
	"time"

	"notes/internal/infrastructure/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

// MetricsMiddleware records Prometheus metrics for each request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &obsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		status := strconv.Itoa(rw.statusCode)
		endpoint := r.URL.Path

		observability.HttpRequestsTotal.WithLabelValues(r.Method, endpoint, status).Inc()
		observability.HttpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(time.Since(start).Seconds())
	})
}

// TracingMiddleware adds OpenTelemetry tracing to each request.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract context from headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		tracer := observability.GetTracer("http-transport")
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path)
		defer span.End()

		// Add attributes to span
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.client_ip", r.RemoteAddr),
		)

		// Update request context with the new span
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

type obsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *obsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
