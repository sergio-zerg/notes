**English** | [Русский](ROADMAP.ru.md)

# Roadmap

Status: ✅ done · 🚧 in progress · 🗓 planned · ❌ not planned

Project-level roadmap. Component-specific implementation notes live in
[`backend/todo.md`](backend/todo.md) and the [frontend README](frontend/README.md).

## Done

- ✅ Go backend: CRUD over notes via **both REST and gRPC**, Clean Architecture / DDD
- ✅ PostgreSQL storage with embedded goose migrations
- ✅ API-key auth (the API refuses to start without keys), per-client rate limiting, body-size limits, CORS, graceful shutdown
- ✅ OpenTelemetry tracing and metrics, structured logging
- ✅ Flutter client (web / iOS / Android): sign in with an API key, search, paging, create and edit notes, light and dark themes
- ✅ One-command stack with Docker Compose (`./up.sh`)
- ✅ CI: gofmt, golangci-lint, govulncheck, unit and integration tests

## Planned

Each of these spans **both** the Go backend (API, storage, migrations) and the
Flutter client (UI):

- 🗓 **Calendar** — a calendar view of notes by date; browse and create notes on a given day
- 🗓 **Reminders** — due dates and reminders on notes, with notifications
- 🗓 **Checklists** — checkboxes / to-do lists inside a note, with per-item checked state
- 🗓 **Markdown support** — write and render note content as Markdown
