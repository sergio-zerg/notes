package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"notes/internal/application"
	"notes/internal/domain"

	"github.com/gorilla/mux"
)

type NoteHandler struct {
	service *application.NoteService
	logger  *slog.Logger
}

func NewNoteHandler(service *application.NoteService) *NoteHandler {
	return &NoteHandler{
		service: service,
		logger:  slog.Default(),
	}
}

// createNoteRequest and updateNoteRequest only decode the payload. The length
// and format rules live in domain.Note.Validate, so the gRPC transport
// enforces exactly the same constraints and there is a single source of truth.
type createNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type updateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// decodeBody reads and decodes a JSON request body.
//
// The read is capped by ValidationMiddleware through http.MaxBytesReader, and a
// body that trips that cap fails as *http.MaxBytesError rather than as a
// syntax error. Reporting that as 413 is the difference between telling a
// client its note is too large and telling it the JSON is malformed, which it
// would answer by shortening the field and failing again.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return err
		}
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return err
	}
	return nil
}

// writeError maps a domain error to an HTTP response. Client errors keep their
// message; anything unexpected is logged and replaced with a generic 500 so
// that internal details are not leaked to callers.
func (h *NoteHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNoteNotFound):
		http.Error(w, "note not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.logger.Error("request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// writeJSON emits a JSON response. Encoding is checked because a failure here
// would otherwise surface as a silently truncated body.
func (h *NoteHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createNoteRequest
	if err := decodeBody(w, r, &req); err != nil {
		return
	}

	note, err := h.service.CreateNote(r.Context(), req.Title, req.Content)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, note)
}

func (h *NoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := noteID(r)
	if err != nil {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}

	note, err := h.service.GetNote(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusOK, note)
}

func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := noteID(r)
	if err != nil {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}

	var req updateNoteRequest
	if err := decodeBody(w, r, &req); err != nil {
		return
	}

	note, err := h.service.UpdateNote(r.Context(), id, req.Title, req.Content)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusOK, note)
}

func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := noteID(r)
	if err != nil {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteNote(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *NoteHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.ListFilter{
		Query:  r.URL.Query().Get("q"),
		Limit:  defaultListLimit,
		Offset: 0,
	}

	// Invalid or out-of-range paging parameters are a client error, but the
	// defaults keep the endpoint usable, so they are clamped rather than rejected.
	if l := r.URL.Query().Get("limit"); l != "" {
		filter.Limit = clampInt(l, 1, maxListLimit, defaultListLimit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		filter.Offset = clampInt(o, 0, maxListLimit, 0)
	}

	notes, err := h.service.ListNotes(r.Context(), filter)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	if notes == nil {
		// Encode an empty slice as [] rather than null.
		notes = []*domain.Note{}
	}

	h.writeJSON(w, http.StatusOK, notes)
}

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// noteID extracts the id path variable. The router pattern already restricts
// it to digits, so the only failure mode is overflow.
func noteID(r *http.Request) (int64, error) {
	return strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
}

func clampInt(raw string, min, max, fallback int) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
