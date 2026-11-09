# 📝 Notes

A small full-stack **notes** application — a Go backend and a Flutter client —
built as a pet project to practise clean architecture, dual **REST + gRPC**
transports, and a production-shaped delivery setup (Docker, migrations, auth,
rate limiting, observability, CI).

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

Defaults: REST on `:8080`, gRPC on `:50051`, web client on `:8081`.
Run `./up.sh` with no Docker, or for host/dev runs, see [`backend/README.md`](backend/README.md).

## Auth & configuration

The API **refuses to start without `API_KEYS`** unless `ALLOW_INSECURE_AUTH=true`
(local experiments only), so a deployment can never come up silently
unauthenticated. Copy the template and edit:

```bash
cp backend/.env.example backend/.env   # .env is gitignored — real secrets never get committed
```

Full configuration reference (database, CORS, rate limits, tracing, shutdown)
is documented inline in [`backend/.env.example`](backend/.env.example).

## Status

Pet project — the focus is architecture and delivery hygiene (layering,
testing, CI, observability), not feature breadth.

## License

[MIT](LICENSE) © sergio-zerg
