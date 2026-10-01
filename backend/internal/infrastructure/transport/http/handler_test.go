package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"notes/internal/application"
	"notes/internal/domain"
	"notes/internal/infrastructure/transport/http/middleware"
)

// stubRepo is a minimal in-memory NoteRepository. It lets the HTTP layer be
// exercised end to end without a database.
type stubRepo struct {
	notes  map[int64]*domain.Note
	nextID int64
	fail   error
}

func newStubRepo() *stubRepo {
	return &stubRepo{notes: map[int64]*domain.Note{}, nextID: 1}
}

func (s *stubRepo) Create(ctx context.Context, n *domain.Note) error {
	if s.fail != nil {
		return s.fail
	}
	n.ID = s.nextID
	s.nextID++
	n.CreatedAt = n.CreatedAt.Add(0)
	s.notes[n.ID] = n
	return nil
}

func (s *stubRepo) GetByID(ctx context.Context, id int64) (*domain.Note, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	n, ok := s.notes[id]
	if !ok {
		return nil, domain.ErrNoteNotFound
	}
	return n, nil
}

func (s *stubRepo) Update(ctx context.Context, n *domain.Note) error {
	if s.fail != nil {
		return s.fail
	}
	if _, ok := s.notes[n.ID]; !ok {
		return domain.ErrNoteNotFound
	}
	s.notes[n.ID] = n
	return nil
}

func (s *stubRepo) Delete(ctx context.Context, id int64) error {
	if s.fail != nil {
		return s.fail
	}
	if _, ok := s.notes[id]; !ok {
		return domain.ErrNoteNotFound
	}
	delete(s.notes, id)
	return nil
}

func (s *stubRepo) List(ctx context.Context, f domain.ListFilter) ([]*domain.Note, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	out := make([]*domain.Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	return out, nil
}

// newTestServer builds the router with the production middleware chain, so
// handler tests also cover body limits, recovery and error mapping.
func newTestServer(repo domain.NoteRepository) http.Handler {
	h := NewNoteHandler(application.NewNoteService(repo))
	r := mux.NewRouter()
	r.Use(middleware.CaptureRouteMiddleware)
	RegisterRoutes(r, h, NewHealthHandler(nil))

	var handler http.Handler = r
	handler = middleware.TracingMiddleware(handler)
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.LoggingMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil)))(handler)
	handler = middleware.RecoveryMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil)))(handler)
	handler = middleware.ValidationMiddleware(handler)
	return handler
}

// do issues a request against the middleware-wrapped handler and returns the
// recorder.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateNote(t *testing.T) {
	r := newTestServer(newStubRepo())

	t.Run("valid payload returns 201", func(t *testing.T) {
		rec := do(t, r, http.MethodPost, "/notes", `{"title":"Valid Title","content":"body"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var note domain.Note
		if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if note.ID == 0 || note.Title != "Valid Title" {
			t.Errorf("unexpected note: %+v", note)
		}
	})

	t.Run("short title is rejected by domain validation", func(t *testing.T) {
		rec := do(t, r, http.MethodPost, "/notes", `{"title":"ab","content":"body"}`)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "title must be at least") {
			t.Errorf("body = %q, want a message about the title length", rec.Body.String())
		}
	})

	t.Run("empty title is rejected", func(t *testing.T) {
		rec := do(t, r, http.MethodPost, "/notes", `{"title":"","content":"body"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("malformed json returns 400", func(t *testing.T) {
		rec := do(t, r, http.MethodPost, "/notes", `{"title":`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	// The handler is wrapped in the same middleware chain as production, so
	// these tests also cover the 413 body limit on write endpoints.
	t.Run("oversized body returns 413", func(t *testing.T) {
		oversize := strings.Repeat("x", middleware.MaxRequestBodySize+1024)
		rec := do(t, r, http.MethodPost, "/notes", oversize)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", rec.Code)
		}
	})

	// A chunked request carries no Content-Length, so the cap is only enforced
	// by MaxBytesReader while the handler reads. That surfaces as a
	// *http.MaxBytesError from the decoder rather than as the middleware's
	// early rejection, and the handler must still report it as 413: answering
	// 400 told the client its JSON was malformed, which it would "fix" by
	// shortening the note and failing again.
	//
	// The body is an unterminated JSON string rather than arbitrary bytes: a
	// payload of bare characters fails on the first byte with a syntax error,
	// before the decoder ever reaches the size limit.
	t.Run("oversized chunked body returns 413, not 400", func(t *testing.T) {
		body := `{"title":"` + strings.Repeat("x", middleware.MaxRequestBodySize+1024)
		req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(body))
		req.ContentLength = -1 // unknown length, as with chunked encoding

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("oversized update returns 413", func(t *testing.T) {
		oversize := strings.Repeat("x", middleware.MaxRequestBodySize+1024)
		rec := do(t, r, http.MethodPut, "/notes/1", oversize)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("PUT status = %d, want 413", rec.Code)
		}
	})

	t.Run("internal error returns 500 without leaking details", func(t *testing.T) {
		repo := newStubRepo()
		repo.fail = errors.New("connection reset by peer: host=db password=hunter2")
		failing := newTestServer(repo)

		rec := do(t, failing, http.MethodPost, "/notes", `{"title":"Valid Title","content":"body"}`)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("response leaked internal error: %q", rec.Body.String())
		}
	})
}

func TestGetNote(t *testing.T) {
	r := newTestServer(newStubRepo())

	created := do(t, r, http.MethodPost, "/notes", `{"title":"Findable","content":"body"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", created.Code, created.Body.String())
	}
	var note domain.Note
	if err := json.Unmarshal(created.Body.Bytes(), &note); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}

	t.Run("existing note returns 200", func(t *testing.T) {
		rec := do(t, r, http.MethodGet, "/notes/1", "")
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("missing note returns 404", func(t *testing.T) {
		rec := do(t, r, http.MethodGet, "/notes/9999", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "note not found") {
			t.Errorf("body = %q", rec.Body.String())
		}
	})

	t.Run("non-numeric id does not match the route", func(t *testing.T) {
		rec := do(t, r, http.MethodGet, "/notes/abc", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404 from the router", rec.Code)
		}
	})
}

func TestUpdateNote(t *testing.T) {
	r := newTestServer(newStubRepo())
	do(t, r, http.MethodPost, "/notes", `{"title":"Original","content":"body"}`)

	t.Run("update returns 200 with new values", func(t *testing.T) {
		rec := do(t, r, http.MethodPut, "/notes/1", `{"title":"Updated Title","content":"new body"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var note domain.Note
		if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if note.Title != "Updated Title" || note.Content != "new body" {
			t.Errorf("unexpected note: %+v", note)
		}
	})

	t.Run("invalid update returns 400", func(t *testing.T) {
		rec := do(t, r, http.MethodPut, "/notes/1", `{"title":"ab","content":"new body"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("update of missing note returns 404", func(t *testing.T) {
		rec := do(t, r, http.MethodPut, "/notes/9999", `{"title":"Valid Title","content":"body"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestDeleteNote(t *testing.T) {
	r := newTestServer(newStubRepo())
	do(t, r, http.MethodPost, "/notes", `{"title":"Doomed","content":"body"}`)

	t.Run("delete returns 204", func(t *testing.T) {
		rec := do(t, r, http.MethodDelete, "/notes/1", "")
		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("deleting twice returns 404", func(t *testing.T) {
		rec := do(t, r, http.MethodDelete, "/notes/1", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404 on the second delete", rec.Code)
		}
	})
}

func TestListNotes(t *testing.T) {
	t.Run("empty store encodes [] not null", func(t *testing.T) {
		rec := do(t, newTestServer(newStubRepo()), http.MethodGet, "/notes", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Errorf("body = %q, want []", got)
		}
	})
}
