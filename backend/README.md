# 📝 Notes API Documentation

A simple, clean, and professional CRUD application for managing notes, built with Go.

## 🏗 Architecture
The project follows **Clean Architecture** and **Domain-Driven Design (DDD)** principles to ensure maintainability, testability, and scalability.

- **Domain Layer**: Pure business logic and entities. No dependencies.
- **Application Layer**: Orchestrates use cases (Services).
- **Infrastructure Layer**: Implementation details (SQLite, HTTP Transport).
- **Principles**: SOLID, KISS, YAGNI.

## 🛠 Tech Stack
- **Language**: Go 1.23+
- **Database**: SQLite (via `go-sqlite3`)
- **Routing**: Gorilla Mux
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
The server will start on `http://localhost:8080`.

## 🔌 API Endpoints

| Method | Endpoint | Description | Request Body | Response |
|--------|----------|-------------|---------------|----------|
| `GET` | `/notes` | List all notes | None | `[Note]` |
| `POST` | `/notes` | Create a note | `{title, content}` | `Note` (201) |
| `GET` | `/notes/{id}` | Get note by ID | None | `Note` (404 if not found) |
| `PUT` | `/notes/{id}` | Update a note | `{title, content}` | `Note` |
| `DELETE` | `/notes/{id}` | Delete a note | None | 204 No Content |

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

## 📜 License
MIT
