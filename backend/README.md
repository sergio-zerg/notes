# 📝 Notes API Documentation

A simple, clean, and professional CRUD application for managing notes, built with Go.

## 🏗 Architecture
The project follows **Clean Architecture** and **Domain-Driven Design (DDD)** principles to ensure maintainability, testability, and scalability.

- **Domain Layer**: Pure business logic and entities. No dependencies.
- **Application Layer**: Orchestrates use cases (Services).
- **Infrastructure Layer**: Implementation details (SQLite, HTTP/gRPC Transport, Observability).
- **Principles**: SOLID, KISS, YAGNI.

## 🛠 Tech Stack
- **Language**: Go 1.23+
- **Database**: SQLite (via `go-sqlite3`)
- **Transport**: REST (Gorilla Mux) & gRPC (Protobuf)
- **Observability**: Prometheus (Metrics) & OpenTelemetry (Tracing)
- **API Spec**: OpenAPI 3.0
- **Testing**: Testify (Mock/Assert)

## 🚀 Quick Start

### Installation
```bash
# Clone the repository
git clone <repo-url>
cd notes

# Install dependencies
go mod tidy
```

### Running the application
```bash
go run cmd/api/main.go
```
- HTTP Server: `http://localhost:8080`
- gRPC Server: `localhost:50051`
- Metrics: `http://localhost:8080/metrics`

## 🔌 API Endpoints

### REST API
| Method | Endpoint | Description | Request Body | Response |
|--------|----------|-------------|---------------|----------|
| `GET` | `/notes` | List all notes | None | `[Note]` |
| `POST` | `/notes` | Create a note | `{title, content}` | `Note` (201) |
| `GET` | `/notes/{id}` | Get note by ID | None | `Note` (404 if not found) |
| `PUT` | `/notes/{id}` | Update a note | `{title, content}` | `Note` |
| `DELETE` | `/notes/{id}` | Delete a note | None | 204 No Content |

### gRPC API
Service: `notes.NoteService`
- `CreateNote(CreateNoteRequest) -> Note`
- `GetNote(GetNoteRequest) -> Note`
- `UpdateNote(UpdateNoteRequest) -> Note`
- `DeleteNote(DeleteNoteRequest) -> DeleteNoteResponse`
- `ListNotes(ListNotesRequest) -> ListNotesResponse`

**Example gRPC call using grpcurl:**
```bash
grpcurl -plaintext -d '{"title": "Hello", "content": "World"}' localhost:50051 notes.NoteService/CreateNote
```

### Note Schema
```json
{
  "id": 1,
  "title": "My Note Title",
  "content": "Note content goes here...",
  "created_at": "2023-10-27T10:00:00Z",
  "updated_at": "2023-10-27T10:05:00Z"
}
```

## 🧪 Testing
Run unit tests for the application layer:
```bash
go test ./internal/application/...
```
Run integration tests (real SQLite):
```bash
go test ./internal/integration_test.go
```

## 🐳 Docker
```bash
docker-compose up -d --build
```

## 📜 License
MIT
