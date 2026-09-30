package http

import (
	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router, h *NoteHandler, health *HealthHandler) {
	r.HandleFunc("/healthz", health.Check).Methods("GET")
	r.HandleFunc("/notes", h.List).Methods("GET")
	r.HandleFunc("/notes", h.Create).Methods("POST")
	r.HandleFunc("/notes/{id:[0-9]+}", h.Get).Methods("GET")
	r.HandleFunc("/notes/{id:[0-9]+}", h.Update).Methods("PUT")
	r.HandleFunc("/notes/{id:[0-9]+}", h.Delete).Methods("DELETE")
}
