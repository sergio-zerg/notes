package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// okHandler records whether the wrapped handler was reached. It is written for
// single-goroutine tests; concurrent tests use it only as a sink.
func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func netAddr(i int) string {
	return fmt.Sprintf("10.0.%d.%d:1234", i/256, i%256)
}

func TestAPIAuthAcceptsValidKey(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*http.Request)
	}{
		{"x-api-key", func(r *http.Request) { r.Header.Set("X-API-Key", "secret") }},
		{"bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }},
		{"whitespace around key", func(r *http.Request) { r.Header.Set("X-API-Key", "  secret ") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := NewAPIKeyAuthenticator([]string{"secret"}, false)
			called := false
			handler := auth.AuthMiddleware(okHandler(&called))

			req := httptest.NewRequest(http.MethodGet, "/notes", nil)
			tc.apply(req)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
			if !called {
				t.Error("the handler did not run for an authenticated request")
			}
		})
	}
}

func TestAPIAuthRejectsMissingOrWrongKey(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*http.Request)
	}{
		{"no header", func(*http.Request) {}},
		{"wrong key", func(r *http.Request) { r.Header.Set("X-API-Key", "nope") }},
		{"empty key", func(r *http.Request) { r.Header.Set("X-API-Key", "") }},
		{"prefix of the real key", func(r *http.Request) { r.Header.Set("X-API-Key", "sec") }},
		{"real key with a suffix", func(r *http.Request) { r.Header.Set("X-API-Key", "secretx") }},
		{"bearer without the scheme", func(r *http.Request) { r.Header.Set("Authorization", "secret") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := NewAPIKeyAuthenticator([]string{"secret"}, false)
			called := false
			handler := auth.AuthMiddleware(okHandler(&called))

			req := httptest.NewRequest(http.MethodGet, "/notes", nil)
			tc.apply(req)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if called {
				t.Error("the handler ran despite the request being unauthenticated")
			}
			if got := rec.Header().Get("WWW-Authenticate"); got == "" {
				t.Error("WWW-Authenticate is not set, so a client cannot tell how to authenticate")
			}
		})
	}
}

// The expected key must not leak into the response body or headers.
func TestAPIAuthErrorDoesNotLeakKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator([]string{"super-secret-key"}, false)
	handler := auth.AuthMiddleware(okHandler(new(bool)))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("X-API-Key", "guess")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, name := range []string{"super-secret-key"} {
		if contains(body, name) {
			t.Errorf("response body contains the configured key: %q", body)
		}
	}
	if contains(rec.Header().Get("WWW-Authenticate"), "super-secret-key") {
		t.Error("WWW-Authenticate contains the configured key")
	}
}

// Health probes must work without a credential, otherwise a container would be
// restarted forever even though the service is fine.
func TestAPIAuthExemptsHealthCheck(t *testing.T) {
	auth := NewAPIKeyAuthenticator([]string{"secret"}, false, "/healthz")
	called := false
	handler := auth.AuthMiddleware(okHandler(&called))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !called {
		t.Error("the health check did not reach the handler")
	}
}

// Exempting /healthz must not exempt everything sharing the prefix.
func TestAPIAuthExemptionIsExact(t *testing.T) {
	auth := NewAPIKeyAuthenticator([]string{"secret"}, false, "/healthz")
	called := false
	handler := auth.AuthMiddleware(okHandler(&called))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/notes", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("a non-exempt path reached the handler")
	}
}

// A trailing comma in API_KEYS must not produce an empty key that lets anyone
// in by omitting the header.
func TestAPIAuthIgnoresEmptyKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator([]string{"", "secret"}, false)
	called := false
	handler := auth.AuthMiddleware(okHandler(&called))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("the handler ran with no key supplied")
	}
}

// With no keys and no opt-in, requests must fail loudly instead of passing.
func TestAPIAuthRefusesWhenUnconfigured(t *testing.T) {
	auth := NewAPIKeyAuthenticator(nil, false)
	called := false
	handler := auth.AuthMiddleware(okHandler(&called))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if called {
		t.Error("the handler ran while authentication was unconfigured")
	}
}

// A second key must also work, so that credentials can be rotated.
func TestAPIAuthAcceptsAnyConfiguredKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator([]string{"old", "new"}, false)
	called := false
	handler := auth.AuthMiddleware(okHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("X-API-Key", "new")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("the second configured key was rejected")
	}
}

func TestRateLimiterRejectsExcess(t *testing.T) {
	limiter := NewRateLimiter(1, 2)
	called := 0
	handler := limiter.RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	codes := make([]int, 3)
	for i := range codes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/notes", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Errorf("first two statuses = %v, want [200 200]", codes[:2])
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Errorf("third status = %d, want 429", codes[2])
	}
	if called != 2 {
		t.Errorf("handler ran %d times, want 2", called)
	}
}

func TestRateLimiterSeparatesClients(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	handler := limiter.RateLimitMiddleware(okHandler(new(bool)))

	send := func(addr string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/notes", nil)
		req.RemoteAddr = addr
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := send("10.0.0.1:1"); got != http.StatusOK {
		t.Errorf("first client = %d, want 200", got)
	}
	// A different client has its own budget.
	if got := send("10.0.0.2:1"); got != http.StatusOK {
		t.Errorf("second client = %d, want 200", got)
	}
	if got := send("10.0.0.1:1"); got != http.StatusTooManyRequests {
		t.Errorf("repeat of first client = %d, want 429", got)
	}
}

// Without a Retry-After a client cannot pace itself and will keep hammering.
func TestRateLimitSetsRetryAfter(t *testing.T) {
	limiter := NewRateLimiter(1, 3)
	handler := limiter.RateLimitMiddleware(okHandler(new(bool)))

	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/notes", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		last = rec
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", last.Code)
	}
	if last.Header().Get("Retry-After") == "" {
		t.Error("Retry-After is not set on a 429 response")
	}
}

// Buckets must not accumulate forever, or a process behind many NATs leaks
// memory one entry per observed address.
func TestRateLimiterSweepsIdleBuckets(t *testing.T) {
	limiter := NewRateLimiter(10, 10)
	handler := limiter.RateLimitMiddleware(okHandler(new(bool)))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.RemoteAddr = "10.0.0.9:1234"
	handler.ServeHTTP(rec(t), req)

	limiter.mu.Lock()
	before := len(limiter.buckets)
	limiter.mu.Unlock()
	if before != 1 {
		t.Fatalf("buckets = %d, want 1", before)
	}

	// A ttl in the past evicts the bucket that was just used.
	limiter.cleanup(-time.Second)

	limiter.mu.Lock()
	after := len(limiter.buckets)
	limiter.mu.Unlock()
	if after != 0 {
		t.Errorf("buckets after sweep = %d, want 0", after)
	}
}

// The cleanup goroutine must actually stop, or every reload leaks one.
func TestRateLimiterStartCleanupStops(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stop := limiter.StartCleanup(ctx, time.Millisecond, time.Millisecond)

	time.Sleep(5 * time.Millisecond)
	stop()
	cancel()
}

// Concurrent clients must not race on the bucket map. The handler counts with
// an atomic, since the race detector would otherwise flag the test itself.
func TestRateLimiterIsConcurrencySafe(t *testing.T) {
	limiter := NewRateLimiter(1000, 1000)

	var served atomic.Int64
	handler := limiter.RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/notes", nil)
			req.RemoteAddr = netAddr(i)
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	if got := served.Load(); got != 20 {
		t.Errorf("served = %d, want 20", got)
	}

	limiter.mu.Lock()
	buckets := len(limiter.buckets)
	limiter.mu.Unlock()
	if buckets != 20 {
		t.Errorf("buckets = %d, want one per client address", buckets)
	}
}

func rec(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	return httptest.NewRecorder()
}
