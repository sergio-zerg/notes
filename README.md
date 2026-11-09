# 📝 Notes

[![CI](https://github.com/sergio-zerg/notes/actions/workflows/ci.yml/badge.svg)](https://github.com/sergio-zerg/notes/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go&logoColor=white)
![Flutter](https://img.shields.io/badge/Flutter-Dart-02569B?logo=flutter&logoColor=white)

A small full-stack **notes** application — a Go backend and a Flutter client —
built as a pet project to practise clean architecture, dual **REST + gRPC**
transports, and a production-shaped delivery setup (Docker, migrations, auth,
rate limiting, observability, CI).

<!-- When you have a screenshot or GIF of the Flutter client, drop it in docs/ and uncomment:
![Notes client](docs/screenshot.png)
-->

## Repository layout

This is a monorepo with two independent components:

| Path | What it is |
|------|------------|
| [`backend/`](backend/README.md) | Go API (Clean Architecture + DDD). CRUD over notes via **both REST and gRPC**, PostgreSQL storage, API-key auth, per-client rate limiting, OpenTelemetry tracing & metrics. |
| [`frontend/`](frontend/README.md) | Flutter client (web / iOS / Android): sign in with an API key, search, page, create and edit notes. |

## Stack

- **Backend:** Go · gorilla/mux (HTTP) · gRPC · pgx + goose migrations · PostgreSQL · OpenTelemetry · token-bucket rate limiting
- **Frontend:** Flutter / Dart
- **Infra:** Docker & Docker Compose · GitHub Actions CI

## Quick start

The whole stack (PostgreSQL + API + Flutter web client) comes up with one command:

```bash
./up.sh                 # build if needed and start everything
./up.sh status          # what is running and on which ports
./up.sh down            # stop, keep the database
```

Once it is up:

| Service | URL |
|---------|-----|
| Web client | http://localhost:8081 |
| REST API | http://localhost:8080 — gRPC on `:50051` |
| Metrics | http://localhost:8080/metrics |

Sign in with the default compose API key **`dev-secret-key-change-me`** (override `API_KEYS` for anything real).

For host/dev runs without Docker, see [`backend/README.md`](backend/README.md).

## Auth & configuration

The API **refuses to start without `API_KEYS`** unless `ALLOW_INSECURE_AUTH=true`
(local experiments only), so a deployment can never come up silently
unauthenticated. Copy the template and edit:

```bash
cp backend/.env.example backend/.env   # .env is gitignored — real secrets never get committed
```

Full configuration reference (database, CORS, rate limits, tracing, shutdown)
is documented inline in [`backend/.env.example`](backend/.env.example).

## What this project practises

- **Dual transports** — one application core served over both REST and gRPC.
- **Clean Architecture / DDD** — dependencies point inward; the domain knows nothing of HTTP or SQL.
- **Delivery hygiene** — embedded migrations, graceful shutdown, API-key auth, per-client rate limiting, body-size limits, CORS.
- **Observability** — OpenTelemetry tracing and metrics, structured logging.
- **Testing & CI** — unit and integration tests across layers, GitHub Actions pipeline.

## Status

Pet project — the focus is architecture and delivery hygiene (layering,
testing, CI, observability), not feature breadth.

## License

[MIT](LICENSE) © sergio-zerg
