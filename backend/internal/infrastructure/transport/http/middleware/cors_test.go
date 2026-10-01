package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newCORSTestChain wires the CORS middleware in front of a handler that records
// whether it was reached, so a test can tell "answered by the middleware" from
// "passed through to the application".
func newCORSTestChain(origins []string, reached *bool) http.Handler {
	return CORSMiddleware(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}))
}

func TestCORSPreflightIsAnsweredWithoutReachingTheApplication(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"http://localhost:8081"}, &reached)

	req := httptest.NewRequest(http.MethodOptions, "/notes", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type, x-api-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if reached {
		t.Error("the preflight reached the application; it carries no credentials and must be answered here")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8081" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the requesting origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods is empty, so the browser cannot issue the real request")
	}
	// The requested headers are echoed, so a client may send X-API-Key without
	// the server having to enumerate it.
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "content-type, x-api-key" {
		t.Errorf("Access-Control-Allow-Headers = %q, want the requested headers echoed", got)
	}
	if rec.Header().Get("Access-Control-Max-Age") == "" {
		t.Error("Access-Control-Max-Age is empty, so every request pays for a fresh preflight")
	}
}

// Without this the browser hides Retry-After, and a rate-limited client has no
// way to learn how long to wait.
func TestCORSExposesRetryAfter(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"http://localhost:8081"}, &reached)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "Retry-After") {
		t.Errorf("Access-Control-Expose-Headers = %q, want it to include Retry-After", got)
	}
}

// A cached response must not be handed to a different origin than the one it
// was computed for.
func TestCORSSetsVaryOnOrigin(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"http://localhost:8081"}, &reached)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to include Origin", got)
	}
}

// An origin outside the allow-list gets no allow header at all, which is what
// makes the browser block the response. The request itself still runs, so
// native clients are unaffected.
func TestCORSOmitsHeaderForUnknownOrigin(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"http://localhost:8081"}, &reached)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent for a disallowed origin", got)
	}
	if !reached {
		t.Error("the request was blocked server-side; CORS is enforced by the browser, not the server")
	}
}

func TestCORSWildcardAllowsAnyOrigin(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"*"}, &reached)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Origin", "http://any-origin.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

// Native clients send no Origin header. Treating that as a rejection would break
// every iOS and Android build, so the request passes through unadorned.
func TestCORSLeavesRequestsWithoutOriginAlone(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"http://localhost:8081"}, &reached)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if !reached {
		t.Error("a request without an Origin header was not forwarded")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent when no origin was sent", got)
	}
}

// A bare OPTIONS request is not a preflight, and treating it as one would hide
// the 405 the router is supposed to return.
func TestCORSPassesPlainOptionsThrough(t *testing.T) {
	var reached bool
	handler := newCORSTestChain([]string{"*"}, &reached)

	req := httptest.NewRequest(http.MethodOptions, "/notes", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !reached {
		t.Error("a plain OPTIONS request was swallowed by the CORS middleware")
	}
}

// A disallowed origin must get no Access-Control-Allow-Origin header at all,
// so the browser hides the response from the page. The request itself still
// runs, which is what leaves non-browser clients unaffected. A wildcard list
// answers with "*" rather than echoing the origin back.
func TestCORSOriginMatching(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		origin  string
		want    string
	}{
		{"listed origin is echoed", []string{"http://localhost:8081"}, "http://localhost:8081", "http://localhost:8081"},
		{"unlisted origin gets no header", []string{"http://localhost:8081"}, "http://localhost:9090", ""},
		{"wildcard answers with a star", []string{"*"}, "http://anything.example", "*"},
		{"wildcard among others", []string{"http://a.example", "*"}, "http://b.example", "*"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var served bool
			h := CORSMiddleware(tc.allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				served = true
			}))

			req := httptest.NewRequest(http.MethodGet, "/notes", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if !served {
				t.Error("the request was blocked; CORS is a browser concern, not access control")
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.want)
			}
		})
	}
}

// A cross-origin response varies by origin, so a shared cache must not hand a
// page the answer computed for another one.
func TestCORSVariesOnOrigin(t *testing.T) {
	h := CORSMiddleware([]string{"*"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}
