package http

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	// Readiness probe: the service is only healthy when it can serve traffic,
	// which requires a working database connection.
	if h.db == nil {
		http.Error(w, "service unavailable: database not configured", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		http.Error(w, "service unavailable: database connection failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// A failed write here means the probe client hung up, which is not worth
	// surfacing: the probe has already been answered.
	_, _ = w.Write([]byte("OK"))
}
