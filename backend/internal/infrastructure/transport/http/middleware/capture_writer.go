package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

var errNotHijackable = errors.New("underlying ResponseWriter does not implement http.Hijacker")

// captureWriter wraps http.ResponseWriter to record the status code that was
// written. It must implement http.Flusher and http.Hijacker, otherwise
// wrapping silently breaks streaming responses (SSE) and connection upgrades
// (WebSocket) further down the chain.
type captureWriter struct {
	http.ResponseWriter
	statusCode  int
	written     int
	wroteHeader bool
}

func newCaptureWriter(w http.ResponseWriter) *captureWriter {
	return &captureWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (cw *captureWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		return
	}
	cw.statusCode = code
	cw.wroteHeader = true
	cw.ResponseWriter.WriteHeader(code)
}

func (cw *captureWriter) Write(b []byte) (int, error) {
	// A Write without an explicit WriteHeader implicitly commits 200.
	if !cw.wroteHeader {
		cw.wroteHeader = true
	}
	n, err := cw.ResponseWriter.Write(b)
	cw.written += n
	return n, err
}

func (cw *captureWriter) StatusCode() int {
	return cw.statusCode
}

// Written reports how many body bytes reached the client.
func (cw *captureWriter) Written() int {
	return cw.written
}

func (cw *captureWriter) Flush() {
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (cw *captureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := cw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errNotHijackable
}

func (cw *captureWriter) Unwrap() http.ResponseWriter {
	return cw.ResponseWriter
}
