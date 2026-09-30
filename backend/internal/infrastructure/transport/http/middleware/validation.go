package middleware

import (
	"net/http"
	"strings"
)

// MaxRequestBodySize caps the body accepted by note write endpoints.
// Content-Length is a client-supplied hint and may be absent (chunked
// encoding), so MaxBytesReader enforces the real ceiling while the handler
// reads, and ContentLength is only used to reject early.
const MaxRequestBodySize = 1 << 20 // 1 MiB

// ValidationMiddleware performs basic checks on incoming requests.
func ValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Applies to POST /notes and PUT/PATCH /notes/{id}. Matching on the
		// prefix keeps the limit working for note updates, whose path differs
		// from the collection endpoint.
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			if strings.HasPrefix(r.URL.Path, "/notes") {
				if r.ContentLength > MaxRequestBodySize {
					http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
					return
				}
				// Enforce the limit for bodies that arrive without a
				// Content-Length, and make the handler observe the error.
				r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
			}
		}
		next.ServeHTTP(w, r)
	})
}
