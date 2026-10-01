package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notes/internal/config"
	"notes/internal/infrastructure/transport/http/middleware"

	"github.com/gorilla/mux"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// chainFor builds the production HTTP chain over a router that records whether
// the handler was reached.
func chainFor(t *testing.T, cfg *config.Config) (http.Handler, *bool) {
	t.Helper()

	reached := new(bool)
	router := mux.NewRouter()
	router.HandleFunc("/notes", func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)

	auth := middleware.NewAPIKeyAuthenticator([]string{"key"}, false, "/healthz")
	_, handler := buildHTTPChain(context.Background(), discardLogger(), router, auth, cfg)
	return handler, reached
}

func testConfig(rateLimitEnabled bool) *config.Config {
	return &config.Config{
		APIKeys:          []string{"key"},
		RateLimitEnabled: rateLimitEnabled,
		// A budget of one request per second with no burst: the second request
		// from the same address is the cheapest way to observe the limiter.
		RateLimitRPS:   1,
		RateLimitBurst: 1,
	}
}

// get issues an authenticated request, since auth sits inside the limiter in
// the chain under test.
func get(handler http.Handler, addr string) int {
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.RemoteAddr = addr
	req.Header.Set("X-API-Key", "key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

// Regression: RATE_LIMIT_ENABLED was parsed and validated in the config package
// and documented in the README, but main.go built the limiter unconditionally,
// so setting it to false changed nothing and every deployment was throttled.
func TestRateLimitEnabledSwitchesLimitingOff(t *testing.T) {
	handler, reached := chainFor(t, testConfig(false))

	for range 5 {
		if code := get(handler, "10.0.0.1:1111"); code != http.StatusOK {
			t.Fatalf("status = %d, want 200 with rate limiting disabled", code)
		}
	}
	if !*reached {
		t.Error("the handler never ran")
	}
}

// The switch must not have removed the limiter altogether: with it on, the
// budget still applies.
func TestRateLimitEnabledAppliesLimiterWhenOn(t *testing.T) {
	handler, _ := chainFor(t, testConfig(true))

	if code := get(handler, "10.0.0.1:2222"); code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", code)
	}
	if code := get(handler, "10.0.0.1:2222"); code != http.StatusTooManyRequests {
		t.Errorf("second status = %d, want 429", code)
	}
}

// The cleanup function must be nil when limiting is off, so that the caller
// does not start a sweep for a limiter that will never see a bucket.
func TestBuildHTTPChainReturnsNoCleanupWhenDisabled(t *testing.T) {
	router := mux.NewRouter()
	auth := middleware.NewAPIKeyAuthenticator([]string{"key"}, false)

	cleanup, _ := buildHTTPChain(context.Background(), discardLogger(), router, auth, testConfig(false))
	if cleanup != nil {
		t.Error("cleanup != nil with rate limiting disabled; the sweep would run for nothing")
	}

	cleanup, _ = buildHTTPChain(context.Background(), discardLogger(), router, auth, testConfig(true))
	if cleanup == nil {
		t.Fatal("cleanup == nil with rate limiting enabled; buckets would never be swept")
	}
	cleanup()
}

// The rate limiter is the outermost layer, so a flood of keyless requests is
// throttled rather than reaching the authenticator. Spending budget on
// unauthenticated traffic is the point: putting auth outside the limiter would
// let anyone without a key drive the service at full CPU indefinitely.
func TestRateLimiterThrottlesUnauthenticatedFlood(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/notes", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)

	cfg := testConfig(true)
	auth := middleware.NewAPIKeyAuthenticator([]string{"key"}, false, "/healthz")
	_, handler := buildHTTPChain(context.Background(), discardLogger(), router, auth, cfg)

	send := func(key string) int {
		req := httptest.NewRequest(http.MethodGet, "/notes", nil)
		req.RemoteAddr = "10.0.0.3:3333"
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// The budget is one, so the first keyless request is refused on its merits
	// and every later one is refused as 429 — the flood is capped either way.
	if got := send(""); got != http.StatusUnauthorized {
		t.Errorf("first keyless status = %d, want 401", got)
	}
	if got := send(""); got != http.StatusTooManyRequests {
		t.Errorf("second keyless status = %d, want 429", got)
	}
	// A correct key cannot buy its way past the budget the flood consumed.
	if got := send("key"); got != http.StatusTooManyRequests {
		t.Errorf("authenticated status after a flood = %d, want 429", got)
	}
}

// A panic inside the chain must still produce a 500 rather than tearing down
// the connection, and the recovery layer sits inside the rate limiter.
func TestChainRecoversPanicUnderRateLimiting(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/notes", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}).Methods(http.MethodGet)

	cfg := testConfig(true)
	auth := middleware.NewAPIKeyAuthenticator([]string{"key"}, false, "/healthz")
	_, handler := buildHTTPChain(context.Background(), discardLogger(), router, auth, cfg)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.RemoteAddr = "10.0.0.4:4444"
	req.Header.Set("X-API-Key", "key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// The body cap must survive the chain, so a write endpoint still answers 413.
func TestChainKeepsBodyLimit(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, middleware.MaxRequestBodySize+1)
		if _, err := r.Body.Read(body); err == nil {
			t.Error("the oversized body was read in full")
		}
		w.WriteHeader(http.StatusCreated)
	}).Methods(http.MethodPost)

	cfg := testConfig(true)
	auth := middleware.NewAPIKeyAuthenticator([]string{"key"}, false, "/healthz")
	_, handler := buildHTTPChain(context.Background(), discardLogger(), router, auth, cfg)

	oversize := strings.Repeat("x", middleware.MaxRequestBodySize+1024)
	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(oversize))
	req.RemoteAddr = "10.0.0.5:5555"
	req.Header.Set("X-API-Key", "key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}
