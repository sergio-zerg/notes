package main

import (
	"fmt"
	"log"
	"net/http"
	"notes/internal/application"
	"notes/internal/infrastructure/persistence"
	"notes/internal/infrastructure/persistence/sqlite"
	httptransport "notes/internal/infrastructure/transport/http"

	"github.com/gorilla/mux"
)

func main() {
	// 1. Infrastructure: SQLite DB
	db, err := persistence.NewSQLiteDB("notes.db")
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// 2. Infrastructure: Persistence Layer
	dao := sqlite.NewNoteDAO(db)
	repo := sqlite.NewSQLiteNoteRepository(dao)

	// 3. Application Layer: Service
	service := application.NewNoteService(repo)

	// 4. Infrastructure: Transport Layer
	handler := httptransport.NewNoteHandler(service)
	router := mux.NewRouter()
	httptransport.RegisterRoutes(router, handler)

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
