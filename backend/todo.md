# TODO: Notes App Implementation

## ✅ Completed

**Foundation**
- [x] Project initialization (Git, Go mod)
- [x] Domain layer: entity, repository interface, errors
- [x] Application layer: `NoteService` use cases
- [x] Infrastructure: Postgres DAO + repository
- [x] Transport: REST (gorilla/mux) handlers and router
- [x] Transport: gRPC handler

**Priority 1 — Core infrastructure**
- [x] gRPC server bootstrap on port 50051
- [x] HTTP and gRPC running concurrently
- [x] Graceful shutdown on SIGINT/SIGTERM for both servers
- [x] Bounded `GracefulStop` with forced `Stop` fallback
- [x] `internal/config` reading ports and DB settings from the environment
- [x] HTTP server timeouts (`ReadHeaderTimeout`, `Read`, `Write`)

**Priority 2 — Reliability & observability**
- [x] Structured logging via `slog`
- [x] Logging middleware
- [x] Recovery middleware for panics
- [x] Validation in the domain layer, shared by REST and gRPC
- [x] Prometheus metrics with route-template labels
- [x] OpenTelemetry tracing
- [x] `SIGTERM`/`SIGINT` handling via `signal.NotifyContext`
- [x] Internal errors logged, never leaked to clients

**Priority 3 — QA & DevOps**
- [x] Unit tests with mocked repository
- [x] Integration tests against real Postgres, behind the `integration` tag
- [x] HTTP handler tests through the full middleware chain
- [x] DAO tests with sqlmock
- [x] Non-root user in the Dockerfile
- [x] Healthcheck in `docker-compose.yml` (real DB ping)
- [x] GitHub Actions CI: fmt, lint, unit, integration, docker build
- [x] `Taskfile` for local workflows
- [x] Migrations embedded with `go:embed`, no runtime file dependency
- [x] README and OpenAPI specification updated
- [x] Metrics and logs labelled with the route template, not the raw path
- [x] Deterministic `ORDER BY` so `OFFSET` paging is stable
- [x] `NotesCreatedTotal` metric removed (would have inverted layer dependencies)
- [x] `task proto:gen` for regenerating protobuf code
- [x] `RATE_LIMIT_ENABLED=false` actually disables limiting on both transports —
      the flag was parsed and validated but never consulted when the chain was
      built, so the documented switch did nothing
- [x] Bucket mechanics shared by both transports in `internal/ratelimit`,
      replacing two near-identical copies that could drift apart
- [x] `Retry-After` advertises the wait for one token (`1/rps`) rather than a
      full bucket refill, which overstated it by up to `burst/rps`; the gRPC
      limiter sends the same value as trailer metadata instead of prose
- [x] An oversized body with no `Content-Length` answers 413 instead of 400 —
      the client was told its JSON was malformed and shortened the note
- [x] `sql.ErrNoRows` matched with `errors.Is`, so a wrapped error is a 404
- [x] Schema: `content` is `NOT NULL`, `id` is an identity column, and search
      has `pg_trgm` GIN indexes

**Priority 4 — Correctness, security, production readiness**
- [x] `UpdateNote` rewritten as a single `UPDATE ... RETURNING` — one round trip,
      no `SELECT`/`UPDATE` window, `created_at` preserved, missing note still 404
- [x] API-key authentication on both transports, constant-time comparison,
      digests held in memory, startup fails without keys
- [x] `/healthz` and `/metrics` exempt from authentication so probes and
      Prometheus need no credential
- [x] Per-client token-bucket rate limiting over REST and gRPC, with `Retry-After`
      and a sweep for idle buckets
- [x] Configuration validated at startup instead of failing on first request
- [x] OTLP trace exporter plus a `ParentBased(TraceIDRatioBased)` sampler
- [x] Batched span export so collector latency stays off the request path
- [x] Tests for the gRPC metrics, auth and rate-limit interceptors
- [x] Tests for config validation and the tracer sampler
- [x] SQLite removed: no driver in `go.mod`, no code, no ignore rules, no local
      `.db` files. Two `modernc.org/sqlite` lines remain in `go.sum` because
      goose v3 lists it for its own tests, and `go.sum` must hash every module in
      the graph. Nothing imports it.

## 🔧 Known Improvements

- [ ] Pagination uses `OFFSET`; keyset pagination would scale better
- [ ] API keys are shared secrets — one leaked key grants full access. Real
      deployments need per-user identities, hashed storage and expiry
- [ ] Rate limiting is per process, so N replicas admit N times the traffic; a
      shared store or an edge limiter would be needed to cap it globally
- [ ] Rate limiting keys on the peer address, so behind a reverse proxy or an
      ingress every client shares one bucket. `X-Forwarded-For` is deliberately
      not trusted because it is client-controlled; a deployment behind a proxy
      needs an explicit list of trusted hop addresses
- [ ] OTLP transport is plaintext (`WithInsecure`); a remote collector needs TLS
- [ ] Traces are sampled per root span, not by service name, so there is no
      separate sampling budget for each dependency
- [ ] gRPC reflection is not registered, so `grpcurl` needs `-proto notes.proto`
- [ ] `CreateNote` could be given an idempotency key to survive client retries
- [ ] Search is covered by `pg_trgm` GIN indexes, but the planner still prefers
      a sequential scan on small tables, which is the cheaper plan there

## 📌 Architecture Reminders

- **SOLID / KISS / YAGNI**
- **Clean Architecture**: Domain → Application → Infrastructure
- **DDD**: repository interfaces live in the domain, implementations in infrastructure
- **Single source of truth** for business rules: the domain layer
