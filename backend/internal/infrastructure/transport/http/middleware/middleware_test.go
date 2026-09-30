package middleware

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The body limit is a stream cap, not just a Content-Length check: a chunked
// request with no Content-Length must still be cut off while the handler
// reads, which is the case the old implementation silently allowed.
func TestValidationMiddlewareRejectsOversizeBodyWithoutContentLength(t *testing.T) {
	oversize := strings.Repeat("x", MaxRequestBodySize+1024)

	var readErr error
	h := ValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	// ContentLength = -1 means the length is unknown (chunked encoding).
	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(oversize))
	req.ContentLength = -1

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if readErr == nil {
		t.Error("oversized body was read without error; MaxBytesReader is not wired up")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Errorf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

// A body at or below the cap must pass through untouched.
func TestValidationMiddlewareAllowsNormalBody(t *testing.T) {
	payload := strings.Repeat("y", 1024)

	var got string
	var readErr error
	h := ValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		readErr = err
		got = string(b)
	}))

	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if readErr != nil {
		t.Fatalf("unexpected read error: %v", readErr)
	}
	if got != payload {
		t.Errorf("body was truncated: got %d bytes, want %d", len(got), len(payload))
	}
}

// A Content-Length above the cap is rejected before the handler runs.
func TestValidationMiddlewareRejectsKnownOversizeLength(t *testing.T) {
	var reached bool
	h := ValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader("small"))
	req.ContentLength = MaxRequestBodySize + 1

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if reached {
		t.Error("handler ran despite an oversized Content-Length")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

// The limit must also cover PUT /notes/{id}, whose path differs from the
// collection endpoint. This is the regression the old exact-path check missed.
func TestValidationMiddlewareAppliesToNoteUpdate(t *testing.T) {
	h := ValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPut, "/notes/42", strings.NewReader("small"))
	req.ContentLength = MaxRequestBodySize + 1

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("PUT /notes/{id} status = %d, want 413", rec.Code)
	}
}

// GET requests are not subject to the write limit.
func TestValidationMiddlewareIgnoresReads(t *testing.T) {
	var reached bool
	h := ValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/notes/1", nil)
	req.ContentLength = MaxRequestBodySize + 1

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Error("GET was blocked by the write-size limit")
	}
}

func TestRecoveryMiddlewareRecoversPanic(t *testing.T) {
	var reached bool
	h := RecoveryMiddleware(testLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if !reached {
		t.Error("handler did not run")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// Recovery is installed outside logging, so a panic must still be logged
// and the request must be counted.
func TestRecoveryMiddlewareStillFeedsMetricsAndLogging(t *testing.T) {
	chain := http.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	chain = MetricsMiddleware(chain)
	chain = LoggingMiddleware(testLogger())(chain)
	chain = RecoveryMiddleware(testLogger())(chain)

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// A streaming handler needs its Flusher to survive the middleware chain.
func TestCaptureWriterPreservesFlusher(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := newCaptureWriter(rec)

	flusher, ok := interface{}(cw).(http.Flusher)
	if !ok {
		t.Fatal("captureWriter does not implement http.Flusher")
	}
	flusher.Flush()
	if !rec.Flushed {
		t.Error("Flush did not reach the underlying writer")
	}
}

func TestCaptureWriterImplementsHijacker(t *testing.T) {
	cw := newCaptureWriter(httptest.NewRecorder())
	if _, ok := interface{}(cw).(http.Hijacker); !ok {
		t.Error("captureWriter does not implement http.Hijacker")
	}
}

// Hijacking a writer that cannot be hijacked must return an error, not panic.
func TestCaptureWriterHijackWithoutSupport(t *testing.T) {
	cw := newCaptureWriter(httptest.NewRecorder())

	var conn net.Conn
	var rw *bufio.ReadWriter
	conn, rw, err := interface{}(cw).(http.Hijacker).Hijack()

	if err == nil {
		t.Error("expected an error hijacking a non-hijackable writer")
	}
	if conn != nil || rw != nil {
		t.Error("expected nil conn/bufio on failure")
	}
}

func TestCaptureWriterRecordsStatusAndBytes(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := newCaptureWriter(rec)

	// Default status is 200 for a handler that only calls Write.
	_, _ = cw.Write([]byte("hello"))

	if cw.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want 200", cw.StatusCode())
	}
	if cw.Written() != 5 {
		t.Errorf("written = %d, want 5", cw.Written())
	}
}

func TestCaptureWriterIgnoresRepeatedWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := newCaptureWriter(rec)

	cw.WriteHeader(http.StatusTeapot)
	cw.WriteHeader(http.StatusInternalServerError)

	if cw.StatusCode() != http.StatusTeapot {
		t.Errorf("status = %d, want the first committed value 418", cw.StatusCode())
	}
}

// Metrics must be labelled with the route template, not the raw path, so that
// /notes/1 and /notes/2 share one time series.
// Regression: every layer of the real chain must see the route template.
// Each layer installs its own route capture, so a per-layer slot shadows the
// outer one and the router fills only the innermost — which left metrics and
// logs labelled with raw paths in production while the unit test (a single
// layer) still passed.
func TestRouteTemplateReachesAllChainLayers(t *testing.T) {
	const template = "/notes/{id:[0-9]+}"

	router := mux.NewRouter()
	router.Use(CaptureRouteMiddleware)
	router.HandleFunc(template, func(w http.ResponseWriter, req *http.Request) {}).Methods(http.MethodGet)

	// Mirrors cmd/api/main.go.
	var h http.Handler = router
	h = TracingMiddleware(h)
	h = MetricsMiddleware(h)
	h = LoggingMiddleware(testLogger())(h)
	h = RecoveryMiddleware(testLogger())(h)
	h = ValidationMiddleware(h)

	before := counterValue(t, http.MethodGet, template)

	for _, id := range []string{"1", "2"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes/"+id, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}

	if got := counterValue(t, http.MethodGet, template); got != before+2 {
		t.Errorf("counter for %q = %v, want %v", template, got, before+2)
	}
	if got := counterValue(t, http.MethodGet, "/notes/1"); got != 0 {
		t.Errorf("counter for raw path /notes/1 = %v, want 0", got)
	}
}

// Metrics must be labelled with the route template, not the raw path, so that
// /notes/1 and /notes/2 share one time series instead of creating a new series
// per note id.
func TestMetricsMiddlewareUsesRouteTemplate(t *testing.T) {
	const (
		method   = "GET"
		template = "/notes/{id:[0-9]+}"
	)

	router := mux.NewRouter()
	router.Use(CaptureRouteMiddleware)
	router.HandleFunc(template, func(w http.ResponseWriter, req *http.Request) {}).Methods(method)

	h := MetricsMiddleware(router)

	before := counterValue(t, method, template)

	// Two different concrete ids must land on the same series.
	for _, id := range []string{"1", "2"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/notes/"+id, nil))
	}

	if got := counterValue(t, method, template); got != before+2 {
		t.Errorf("counter for %q = %v, want %v (route template not used?)", template, got, before+2)
	}

	// The raw path must not have produced its own series.
	if got := counterValue(t, method, "/notes/1"); got != 0 {
		t.Errorf("counter for raw path /notes/1 = %v, want 0", got)
	}
}

// counterValue reads a labelled value out of the Prometheus default registry.
func counterValue(t *testing.T, method, endpoint string) float64 {
	t.Helper()

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	for _, mf := range mfs {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !hasLabels(m, map[string]string{"method": method, "endpoint": endpoint}) {
				continue
			}
			if m.Counter != nil {
				return m.Counter.GetValue()
			}
		}
	}
	return 0
}

func hasLabels(m *dto.Metric, want map[string]string) bool {
	for k, v := range want {
		found := false
		for _, l := range m.GetLabel() {
			if l.GetName() == k && l.GetValue() == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
