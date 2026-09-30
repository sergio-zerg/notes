package middleware

import (
	"net/http"
	"strings"
)

const (
	// corsPreflightMaxAge tells the browser how long it may cache a preflight
	// result. Ten minutes keeps the handshake off the critical path without
	// leaving a stale allow-list behind after the configuration changes.
	corsPreflightMaxAge = "600"

	// corsAllowedMethods lists the verbs the notes API actually serves. An
	// explicit list is narrower than reflecting the requested method back.
	corsAllowedMethods = "GET, POST, PUT, DELETE, OPTIONS"

	// corsDefaultAllowHeaders is used when a preflight does not say which
	// headers it needs. X-API-Key and Authorization are the two documented
	// ways to authenticate; Content-Type is required by the JSON endpoints.
	corsDefaultAllowHeaders = "Content-Type, X-API-Key, Authorization"

	// corsExposeHeaders names the response headers a browser client cannot
	// read unless they are exposed. Retry-After matters most: a rate-limited
	// Flutter web client paces its retries from it, and without this it can
	// only guess.
	corsExposeHeaders = "Retry-After, WWW-Authenticate"
)

// CORSMiddleware answers browser preflights and marks cross-origin responses as
// readable by the requesting page.
//
// A browser-based client — a Flutter web build served from its own dev server
// or from any other host — is a different origin from this API, and the browser
// hides the response from the application entirely unless these headers are
// present. Native clients (iOS, Android) are not browsers and ignore the whole
// mechanism, which is why this only ever adds headers.
//
// Two details are load-bearing:
//
//   - Preflights are answered here and never forwarded. They carry no
//     credentials, so routing them through the authenticator would reject every
//     cross-origin write with a 401, and routing them through the rate limiter
//     would spend a client's token budget on the handshake.
//   - Vary: Origin is always set. The response body differs by origin, so a
//     shared cache must not hand a page the answer computed for another one.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowed[origin]; !ok {
					// No header at all: the browser then blocks the response,
					// which is the outcome a disallowed origin must get. The
					// request still runs, so non-browser clients are
					// unaffected.
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if origin != "" {
				// The allow-list changes which response a given URL produces,
				// so caches have to key on the origin as well.
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				// Echoing the requested headers lets a client add a header
				// without a server change, and the browser still refuses
				// anything the response does not allow.
				if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
					w.Header().Set("Access-Control-Allow-Headers", requested)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", corsDefaultAllowHeaders)
				}
				w.Header().Set("Access-Control-Max-Age", corsPreflightMaxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OriginAllowed reports whether the given Origin would be granted access. It
// exists for tests and for callers that need to explain a rejection before the
// request is made.
func OriginAllowed(allowedOrigins []string, origin string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}
