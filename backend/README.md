# 📝 Notes API

A CRUD service for managing notes, built with Go following Clean Architecture and Domain-Driven Design.

## 🏗 Architecture

Dependencies point inwards only: `domain` imports nothing from the project, `application` depends on `domain`, and `infrastructure` implements the interfaces the inner layers define.

```
cmd/api/                 composition root: config, tracing, DB, both servers, graceful shutdown
internal/
  domain/                Note entity, repository interface, errors, validation
  application/           NoteService — use cases, no HTTP or SQL knowledge
  config/                environment-based configuration
  infrastructure/
    persistence/         pgx connection, goose migrations (embedded), postgres DAO + repository
    transport/http/      gorilla/mux handlers, router, health probe
    transport/grpc/      gRPC handler
    transport/http/middleware/  logging, recovery, metrics, tracing, body limit, API keys, rate limit, CORS
    transport/grpc/middleware/  metrics, API keys, rate limit
    observability/       Prometheus metrics, OpenTelemetry tracer
api/
  proto/                 notes.proto and generated code
  openapi.yaml           REST contract
```

**Principles**: SOLID, KISS, YAGNI. Validation lives in the domain, so the REST and gRPC transports enforce identical rules.

## 🛠 Tech Stack

- **Language**: Go 1.26
- **Database**: PostgreSQL 16 (pgx v5), migrations via goose, embedded with `go:embed`
- **Transport**: REST (gorilla/mux) & gRPC (Protobuf)
- **Observability**: Prometheus metrics & OpenTelemetry tracing (stdout or OTLP collector)
- **Security**: shared-secret API keys, per-client token-bucket rate limiting
- **API Spec**: OpenAPI 3.0 — `api/openapi.yaml`
- **Testing**: Testify, sqlmock, `net/http/httptest`

## 🚀 Quick Start

```bash
git clone <repo-url>
cd notes
go mod tidy

# Run against a local Postgres
docker compose up -d db
API_KEYS=dev-secret-key go run ./cmd/api
```

Or run the whole stack:

```bash
docker compose up -d --build
```

| Endpoint | Address |
|----------|---------|
| REST API | `http://localhost:8080` |
| gRPC API | `localhost:50051` |
| Health | `http://localhost:8080/healthz` |
| Metrics | `http://localhost:8080/metrics` |

## ⚙️ Configuration

All settings come from the environment, with defaults suitable for local development.

| Variable | Default | Purpose |
|----------|---------|---------|
| `HTTP_PORT` | `:8080` | REST listen address |
| `GRPC_PORT` | `:50051` | gRPC listen address |
| `DB_HOST` | `localhost` | Postgres host |
| `DB_PORT` | `5432` | Postgres port. Under Compose this moves the published host port only |
| `DB_USER` | `postgres` | Postgres user |
| `DB_PASS` | `password` | Postgres password |
| `DB_NAME` | `notes_db` | Database name |
| `API_KEYS` | — | Comma-separated keys for the notes endpoints. **Required**: startup fails without it |
| `ALLOW_INSECURE_AUTH` | `false` | Permits running with no keys. Development only |
| `RATE_LIMIT_ENABLED` | `true` | Turn per-client rate limiting off |
| `RATE_LIMIT_RPS` | `20` | Sustained requests per second per client |
| `RATE_LIMIT_BURST` | `40` | Extra requests allowed in a burst |
| `CORS_ALLOWED_ORIGINS` | `*` | Browser origins allowed to read responses. Web clients only |
| `TRACE_EXPORTER` | `stdout` | `stdout`, `otlp` or `none` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | Collector address when `TRACE_EXPORTER=otlp` |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | Fraction of root spans recorded, within [0,1] |
| `SHUTDOWN_TIMEOUT` | `10s` | Grace period for in-flight requests |

Configuration is validated at startup: an unusable combination — no API keys, a
non-positive rate limit, an unknown exporter, a sample ratio outside [0,1], a
non-positive shutdown budget — fails immediately instead of on the first
request.

### Using a `.env` file

`.env.example` lists every variable with a short explanation. Copy it and edit:

```bash
cp .env.example .env
docker compose up -d          # Compose substitutes ${VAR} from .env
```

`.env` is gitignored, so real secrets stay out of commits; `.env.example` is
tracked and safe to share. Note the scope, it is narrower than it looks:

- `docker compose up` reads `.env` automatically, and nothing else does.
- `go run ./cmd/api` does **not** read it. The application only calls
  `os.Getenv`, so for a host run use `set -a; . ./.env; set +a`.
- CI has its own `env` block in `.github/workflows/ci.yml` and ignores `.env`.

`DB_USER`, `DB_PASS` and `DB_NAME` feed both the Postgres container and the
application, so credentials are defined once. `DB_PORT` moves only the
published host port: inside the Compose network the app still reaches the
database on the image's own 5432, by service name.

## 🔐 Authentication

The notes endpoints require a shared secret. Send it as `X-API-Key` or as
`Authorization: Bearer <key>`; over gRPC, as `x-api-key` metadata. Keys are
compared in constant time and stored as SHA-256 digests.

```bash
curl localhost:8080/notes -H 'X-API-Key: dev-secret-key'
```

`GET /healthz` and `GET /metrics` stay open, so orchestrator probes and
Prometheus scrapes need no credential. Unauthenticated requests get `401` over
REST and `Unauthenticated` over gRPC.

## 🚦 Rate Limiting

Each client address gets a token bucket refilling at `RATE_LIMIT_RPS` up to
`RATE_LIMIT_BURST`. Exhausted clients receive `429` with a `Retry-After` header
over REST and `ResourceExhausted` over gRPC. Buckets idle for longer than ten
minutes are swept.

## 🌐 CORS

Browsers enforce same-origin policy, so a web client served from its own origin
is refused the response unless the server opts it in. A Flutter web build hits
this immediately: `flutter run -d chrome` serves on a different port than the
API, and the browser blocks every response before the app sees it.

CORS is off the critical path for everyone else. Native iOS and Android clients
are not browsers, requests without an `Origin` header are untouched, and the
gRPC transport has no such concept.

```bash
# Development: the default wildcard already covers Flutter's random dev port
CORS_ALLOWED_ORIGINS='*'

# Production: pin what actually serves your client
CORS_ALLOWED_ORIGINS='https://notes.example.com'
```

Preflights are answered by the middleware itself and never reach the
authenticator or the rate limiter — they carry no credentials, and counting them
would spend a client's budget on the handshake. `Retry-After` and
`WWW-Authenticate` are listed in `Access-Control-Expose-Headers`, without which
a browser hides them and a rate-limited client cannot pace its retries.
`Vary: Origin` is always set so a shared cache cannot hand one origin a response
computed for another.

Entries must be bare origins — scheme, host, optional port. A trailing slash or
a path is rejected at startup, because the browser compares the entry against
the `Origin` header verbatim and such a value would never match.

## 🔌 API

### REST

| Method | Endpoint | Description | Success |
|--------|----------|-------------|---------|
| `GET` | `/notes` | List notes (`q`, `limit`, `offset`) | 200 |
| `POST` | `/notes` | Create a note | 201 |
| `GET` | `/notes/{id}` | Get a note | 200 |
| `PUT` | `/notes/{id}` | Update a note | 200 |
| `DELETE` | `/notes/{id}` | Delete a note | 204 |
| `GET` | `/healthz` | Readiness (pings the database) | 200 |
| `GET` | `/metrics` | Prometheus exposition | 200 |

```bash
curl -X POST localhost:8080/notes \
  -H 'X-API-Key: dev-secret-key' \
  -H 'Content-Type: application/json' \
  -d '{"title":"My Note Title","content":"Some content"}'

curl 'localhost:8080/notes?q=note&limit=10' -H 'X-API-Key: dev-secret-key'
```

A `Note` looks like this:

```json
{
  "id": 1,
  "title": "My Note Title",
  "content": "Note content goes here...",
  "created_at": "2026-09-30T10:00:00Z",
  "updated_at": "2026-09-30T10:05:00Z"
}
```

### Validation

Enforced in `domain.Note.Validate`, identical for both transports:

- `title` required, 3–100 characters (counted in **runes**, so Cyrillic titles are measured fairly)
- `content` required

Violations return `400` over REST and `InvalidArgument` over gRPC. Request bodies over 1 MiB return `413`.

### gRPC

Service `notes.NoteService`:

- `CreateNote(CreateNoteRequest) → Note`
- `GetNote(GetNoteRequest) → Note`
- `UpdateNote(UpdateNoteRequest) → Note`
- `DeleteNote(DeleteNoteRequest) → DeleteNoteResponse`
- `ListNotes(ListNotesRequest) → ListNotesResponse` — supports `query`, `limit`, `offset`

The server does **not** expose gRPC reflection, so `grpcurl` needs the proto file and the API key:

```bash
grpcurl -plaintext -import-path api/proto -proto notes.proto \
  -H 'x-api-key: dev-secret-key' \
  -d '{"title":"Hello","content":"World"}' \
  localhost:50051 notes.NoteService/CreateNote

grpcurl -plaintext -import-path api/proto -proto notes.proto \
  -H 'x-api-key: dev-secret-key' \
  -d '{"query":"Hello","limit":10}' \
  localhost:50051 notes.NoteService/ListNotes
```

## 📊 Observability

`/metrics` exposes HTTP request counters and latency histograms plus gRPC
counters labelled by method and status code. Series carry the route template
(`/notes/{id:[0-9]+}`) rather than concrete paths, so traffic to individual
notes does not create unbounded cardinality. The scrape itself is served
outside the instrumented chain, so Prometheus does not measure its own polling.

Traces are exported to stdout by default. Point them at a collector instead:

```bash
# A ready-made collector config ships in otel-collector.yaml
docker run --rm -p 4317:4317 \
  -v ./otel-collector.yaml:/etc/otelcol-contrib/config.yaml \
  otel/opentelemetry-collector-contrib:latest

TRACE_EXPORTER=otlp OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
OTEL_TRACES_SAMPLER_ARG=0.1 go run ./cmd/api
```

Sampling is `ParentBased(TraceIDRatioBased(ratio))`, so a trace sampled upstream
stays sampled here, and `OTEL_TRACES_SAMPLER_ARG=1.0` in production records
every request. Spans are batched rather than exported synchronously, so an
export round trip never lands on the request that produced the span.

## 🧪 Testing

```bash
task test:unit          # mocked repository, no database
task test:integration   # real Postgres, build tag "integration"
task proto:gen          # regenerate protobuf code after editing notes.proto
task vendor             # go mod tidy + go mod vendor
task ci                 # fmt + lint + vendor:check + unit + integration + docker build
```

## 🔐 Dependency Security

Dependencies are committed to `vendor/`, so a build needs no network access and
picks up exactly the reviewed tree. `task ci` fails if `vendor/` drifts from
`go.mod`.

`govulncheck ./...` runs as its own CI step. It reports only vulnerabilities
whose affected symbols are actually reachable from the code, which is what
separates a real exposure from a scary version number in a lockfile:

```bash
govulncheck ./...
```

It has already earned its place here. `google.golang.org/grpc v1.84.0` carries
[GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443) (CVE-2026-84445), a panic
in the xDS server routing interceptor that terminates the process when a request
arrives without `:authority` or `Host`. This service does not configure xDS, so
the panic path itself is unreachable — but govulncheck still traced the call
graph through `grpc.Server.Serve` to `transport.http2Server.HandleStreams` and
reported the module as affected, which is the conservative answer worth acting
on. The pin in `go.mod` moves to the commit that carries the fix.

Integration tests are behind the `integration` tag so `go test ./...` works without a database. They read connection settings from the environment and skip when unset:

```bash
NOTES_TEST_DB_HOST=localhost go test -tags=integration -v ./internal/
```

## 📦 Dependencies

`vendor/` is committed. A build therefore needs no network access and uses
exactly the tree that was reviewed, which matters most for the Docker image.
Refresh it with `task vendor` after changing imports, and let `task ci`'s
`vendor:check` catch a forgotten refresh.

## 🐳 Docker

```bash
docker compose up -d --build
docker compose logs -f notes-app
docker compose down -v
```

The runtime image runs as a non-root user, is built with `CGO_ENABLED=0`, and carries the migration SQL inside the binary.

## 📜 License

MIT
