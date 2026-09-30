# TODO: Notes App Implementation

## ✅ Completed
- [x] Project Initialization (Git, Go mod)
- [x] Domain Layer (Entities, Repository Interface)
- [x] Infrastructure: Persistence (SQLite DB, DAO, Repository)
- [x] Application Layer (NoteService)
- [x] Infrastructure: Transport (HTTP Handlers, Router)
- [x] API Specification (OpenAPI)
- [x] Main Entry Point (HTTP server bootstrap)
- [x] Unit Testing (Application layer with mocks)
- [x] Dockerization (Dockerfile, Docker Compose)
- [x] Documentation (README.md)
- [x] gRPC Protocol Definition (`notes.proto`)
- [x] gRPC Code Generation (Protoc)
- [x] gRPC Handler Implementation

## ⏳ In Progress / Pending
### Priority 1: Core Infrastructure (Crucial)
- [ ] **Bootstrapping gRPC Server**
    - [ ] Update `cmd/api/main.go` to start gRPC server on port 50051
    - [ ] Ensure both HTTP and gRPC servers run concurrently (goroutines)
- [ ] **Graceful Shutdown**
    - [ ] Implement signal handling (SIGINT/SIGTERM) in `main.go`
    - [ ] Implement graceful stop for `http.Server` and `grpc.Server`
- [ ] **Configuration Management**
    - [ ] Create `internal/config` package
    - [ ] Support environment variables for ports and DB path

### Priority 2: Reliability & Observability (Engineering)
- [ ] **Structured Logging**
    - [ ] Replace `log` with `slog` (Go standard library)
- [ ] **HTTP Middleware**
    - [ ] Add Logging middleware (request/response logging)
    - [ ] Add Recovery middleware (panic handling)
- [ ] **Input Validation**
    - [ ] Implement basic validation for Note entities (length, format)

### Priority 3: QA & DevOps (Polishing)
- [ ] **Integration Testing**
    - [ ] Implement tests with a real SQLite database
    - [ ] E2E test: gRPC $\rightarrow$ DB $\rightarrow$ REST
- [ ] **Docker Hardening**
    - [ ] Add non-root user to `Dockerfile`
    - [ ] Add `healthcheck` to `docker-compose.yml`
- [ ] **Documentation Update**
    - [ ] Add gRPC usage examples to `README.md`

## 📌 Architecture Reminders
- **SOLID/KISS/YAGNI**
- **Clean Architecture**: Domain $\rightarrow$ Application $\rightarrow$ Infrastructure
- **DDD**: Repository interfaces in Domain, implementations in Infrastructure
