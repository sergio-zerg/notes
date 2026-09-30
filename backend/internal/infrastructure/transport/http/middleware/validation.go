package middleware

import (
	"net/http"
)

// ValidationMiddleware performs basic checks on incoming requests.
func ValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate POST and PUT requests to /notes
		if (r.Method == http.MethodPost || r.Method == http.MethodPut) && r.URL.Path == "/notes" {
			// This is a simple check. For complex validation,
			// it's better to do it in the Application layer.
			// Here we just check for a reasonable Content-Length to avoid huge payloads.
			if r.ContentLength > 1*1024*1024 { // 1MB limit
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
