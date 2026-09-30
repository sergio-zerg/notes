package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// LoggingMiddleware logs the details of each incoming HTTP request.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			cw := newCaptureWriter(w)

			r, slot := WithRouteCapture(r)
			next.ServeHTTP(cw, r)

			logger.Info("request processed",
				"method", r.Method,
				"route", routeLabel(slot, r),
				"uri", r.RequestURI,
				"status", cw.StatusCode(),
				"bytes", cw.Written(),
				"duration", time.Since(start),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
