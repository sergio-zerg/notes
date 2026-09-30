package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"notes/internal/infrastructure/observability"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// routeKey carries a shared slot for the matched route pattern.
//
// The pattern is only known once gorilla/mux has matched the request, which
// happens inside the router — after an outer middleware has already been
// entered. Putting a *string in the request context lets the inner mux
// middleware (see CaptureRouteMiddleware) write the pattern and the outer
// middleware read it after the handler returns.
type routeKey struct{}

// WithRouteCapture returns a request carrying a route slot, plus the pointer
// to that slot. Read the pointer after the next handler has run.
//
// The slot is created only by the outermost caller. Nested middleware reuses
// the slot already in the context, so a single CaptureRouteMiddleware inside
// the router fills it for every layer of the chain. Creating a fresh slot per
// layer would shadow the outer one and leave the outer middlewares reading an
// empty value.
func WithRouteCapture(r *http.Request) (*http.Request, *string) {
	if slot, ok := r.Context().Value(routeKey{}).(*string); ok {
		return r, slot
	}
	slot := new(string)
	return r.WithContext(context.WithValue(r.Context(), routeKey{}, slot)), slot
}

// CaptureRouteMiddleware is registered on the router via mux.Router.Use. It
// runs after route matching, so the pattern is available and the context
// written by WithRouteCapture can be filled in.
func CaptureRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slot, _ := r.Context().Value(routeKey{}).(*string)
		if slot == nil {
			next.ServeHTTP(w, r)
			return
		}
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				*slot = tmpl
			}
		}
		if *slot == "" {
			*slot = r.URL.Path
		}
		next.ServeHTTP(w, r)
	})
}

// routeLabel returns the captured route pattern, falling back to the raw path.
func routeLabel(slot *string, r *http.Request) string {
	if slot != nil && *slot != "" {
		return *slot
	}
	return r.URL.Path
}

// MetricsMiddleware records Prometheus metrics for each request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cw := newCaptureWriter(w)

		r, slot := WithRouteCapture(r)
		next.ServeHTTP(cw, r)

		route := routeLabel(slot, r)

		observability.HttpRequestsTotal.
			WithLabelValues(r.Method, route, strconv.Itoa(cw.StatusCode())).
			Inc()
		observability.HttpRequestDuration.
			WithLabelValues(r.Method, route).
			Observe(time.Since(start).Seconds())
	})
}

// TracingMiddleware adds OpenTelemetry tracing to each request.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// The full span name needs the matched route, which is only known once
		// the router has run, so the name is completed after next.
		tracer := observability.GetTracer("http-transport")
		ctx, span := tracer.Start(ctx, r.Method)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.client_ip", r.RemoteAddr),
		)

		cw := newCaptureWriter(w)
		r, slot := WithRouteCapture(r.WithContext(ctx))
		next.ServeHTTP(cw, r)

		span.SetName(r.Method + " " + routeLabel(slot, r))
		span.SetAttributes(attribute.Int("http.status_code", cw.StatusCode()))

		if cw.StatusCode() >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(cw.StatusCode()))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	})
}
