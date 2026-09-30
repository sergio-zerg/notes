package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// APIKeyAuthenticator validates the API key supplied with each request.
//
// Keys are stored as SHA-256 digests rather than in clear text: a heap dump or
// a stray log line must not hand out working credentials. Comparison uses
// subtle.ConstantTimeCompare so that a rejected key reveals nothing about how
// many leading characters were correct.
type APIKeyAuthenticator struct {
	// digests holds one precomputed hash per configured key.
	digests [][sha256.Size]byte
	// allowEmpty keeps every request when no keys are configured. It exists
	// only for local development and is set explicitly by the caller.
	allowEmpty bool
	// exempt lists paths served without a key, so that liveness probes and
	// Prometheus scrapes do not need credentials.
	exempt map[string]struct{}
}

// NewAPIKeyAuthenticator builds an authenticator for the given keys.
//
// When keys is empty the authenticator rejects every request unless
// allowEmpty is set. Silently allowing all traffic because a variable happened
// to be unset is the failure mode worth designing against.
func NewAPIKeyAuthenticator(keys []string, allowEmpty bool, exemptPaths ...string) *APIKeyAuthenticator {
	auth := &APIKeyAuthenticator{
		digests:    make([][sha256.Size]byte, 0, len(keys)),
		allowEmpty: allowEmpty,
		exempt:     make(map[string]struct{}, len(exemptPaths)),
	}
	for _, key := range keys {
		if key == "" {
			// An empty key must never be accepted: without this guard a
			// trailing comma in API_KEYS would authenticate anyone who
			// omits the header.
			continue
		}
		auth.digests = append(auth.digests, sha256.Sum256([]byte(key)))
	}
	for _, path := range exemptPaths {
		auth.exempt[path] = struct{}{}
	}
	return auth
}

// Enabled reports whether any key is configured.
func (a *APIKeyAuthenticator) Enabled() bool {
	return len(a.digests) > 0 || a.allowEmpty
}

// AuthMiddleware rejects requests that do not carry an accepted API key.
//
// The key may arrive as the X-API-Key header or as an Authorization header of
// the form "Bearer <key>", since HTTP clients and gRPC metadata carry the two
// in different shapes.
func (a *APIKeyAuthenticator) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.exempt[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}

		if !a.Enabled() {
			writeAuthError(w, http.StatusInternalServerError, "authentication is not configured")
			return
		}

		if len(a.digests) == 0 {
			// allowEmpty with no keys: the operator asked for an open API.
			next.ServeHTTP(w, r)
			return
		}

		presented := extractAPIKey(r.Header.Get("X-API-Key"), r.Header.Get("Authorization"))
		if !a.matches(presented) {
			// The challenge tells a client how to authenticate without
			// disclosing anything about the expected key.
			w.Header().Set("WWW-Authenticate", `Bearer realm="notes"`)
			writeAuthError(w, http.StatusUnauthorized, "valid API key required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// matches compares the presented key against every configured one, always
// checking all of them so that timing does not depend on which key matched.
func (a *APIKeyAuthenticator) matches(presented string) bool {
	if presented == "" {
		return false
	}
	candidate := sha256.Sum256([]byte(presented))

	var found int
	for _, digest := range a.digests {
		// Accumulate rather than return early.
		found |= subtle.ConstantTimeCompare(candidate[:], digest[:])
	}
	return found == 1
}

// extractAPIKey pulls the key out of either supported header form.
func extractAPIKey(apiKeyHeader, authorization string) string {
	if apiKeyHeader != "" {
		return strings.TrimSpace(apiKeyHeader)
	}
	if after, found := strings.CutPrefix(authorization, "Bearer "); found {
		return strings.TrimSpace(after)
	}
	return ""
}

// writeAuthError emits a minimal JSON body. The message is always a literal
// from this package, so it never carries request data.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
