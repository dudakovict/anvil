# anvil

Production-ready starter for Go backend services, structured as a **hexagonal
(ports & adapters) architecture**. Clone it, rename the module, and you have a
working HTTP API with the operational fundamentals already in place:

- **HTTP server hardening** — `http.TimeoutHandler` bounds request duration; `ReadTimeout`,
  `WriteTimeout`, `IdleTimeout` all set explicitly
- **Graceful shutdown** — SIGINT/SIGTERM drains in-flight requests, zero-downtime rolling updates
- **Structured logging** — stdlib `log/slog` (JSON) with a `ContextHandler` that automatically
  enriches every `slog.XxxContext(ctx, ...)` call with `request_id` and `trace_id`/`span_id`
- **OpenTelemetry tracing & metrics** — server spans (otelchi), SQL query spans (GORM OTel plugin),
  outbound call spans (otelhttp), W3C `traceparent` propagation; HTTP request duration/active
  requests metrics per route + Go runtime metrics; OTLP export gated by `OTEL_ENABLED`, local
  Grafana UI via `grafana/otel-lgtm` in docker-compose (`http://localhost:3000`)
- **Request ID middleware** — reuses `X-Request-Id` or generates a UUID; propagated via context,
  echoed in the response, ready to forward to internal services
- **Access log middleware** — method, path, status, duration per request
- **JWT auth + roles** — `POST /auth/login` verifies argon2id credentials and issues an HS256
  token carrying the user's role; `web.Auth` protects route groups and puts the claims in the
  context, `web.Authorize` gates admin routes, handlers check resource ownership (`canAccess`)
- **API hardening** — OWASP security headers on every response, per-IP rate limit on the login
  endpoint (argon2id makes each attempt expensive), govulncheck in CI
- **Request validation** — [go-playground/validator](https://github.com/go-playground/validator)
  tags on DTOs, checked automatically in `web.Decode`/`web.DecodeQuery` (a `Validate()` method
  overrides for cross-field rules); failures return a 400 with a per-field `fields` list
- **Config from environment** — [caarlos0/env](https://github.com/caarlos0/env) with defaults,
  required fields and cross-field validation at boot
- **Postgres + migrations** — [GORM](https://gorm.io) (pgx-backed) with OTel query tracing;
  [goose](https://github.com/pressly/goose) SQL migrations embedded in the binary and applied on boot
- **Swagger** — [swaggo](https://github.com/swaggo/swag) docs generated from annotations, served
  at `/swagger/`
- **Example domain** (`user`) — wired through every layer of the hexagon, ready to copy
- **Integration testing** — throwaway Postgres containers via `platform/docker`
- **Tooling** — Makefile, golangci-lint, multi-stage distroless Dockerfile, docker-compose,
  GitHub Actions (lint + test + govulncheck on every push and PR, GHCR image build on the
  default branch), version stamping
  via ldflags (`service.version` in traces, logged at boot)

## Quickstart

```bash
make docker-up            # builds the image, starts postgres + app
curl localhost:8080/healthz
curl -X POST localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"john.doe@example.com","name":"John Doe","password":"s3cret-pass"}'
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"john.doe@example.com","password":"s3cret-pass"}' | jq -r .access_token)
curl localhost:8080/api/v1/me -H "Authorization: Bearer $TOKEN"
open http://localhost:8080/swagger/    # interactive API docs
```

## Local development

```bash
make tools                # install swag + goose + mockgen (go tool) and golangci-lint
make db-up                # start only postgres in docker
make docker-watch         # full stack with rebuild-on-save (compose watch)
cp .env.example .env      # then: set -a; . ./.env; set +a   (or use direnv)
make run                  # run the app on the host
make test                 # unit + integration tests (-race); integration spins up its own container
make lint                 # golangci-lint
make audit                # everything CI checks: tidy diff, verify, tests, lint, vulncheck
make swagger              # regenerate docs/ after changing annotations
```

`make help` lists all targets.

## Architecture

Hexagonal / ports & adapters: the business logic (`core`) is isolated from
technology; adapters plug into it from the outside.

```
cmd/api/main.go        composition root — the ONLY place that knows every layer
migrations/            versioned SQL schema (goose), embedded and applied on boot
internal/
├── core/              the hexagon: no HTTP, no SQL, no technology
│   ├── domain/        business entities + domain errors (User, ErrUserNotFound)
│   ├── port/          contracts: in-ports (UserService) and out-ports (UserRepository)
│   └── service/       use-case implementations = the business logic
├── adapter/
│   ├── rest/          DRIVING adapter: handlers, DTOs, router, server
│   └── postgres/      DRIVEN adapter: GORM repositories
├── config/            env → struct for THIS service, validated at boot
└── platform/          generic infrastructure, knows nothing about domains
    ├── web/           HTTP plumbing: web.Handler with centralized rendering + error
    │                  mapping (Handle), middleware, body + query decode — works with any router
    ├── argon2id/      password hashing (PHC string format, OWASP params)
    ├── auth/          JWT issue/verify + claims (user id, role) context propagation
    ├── database/      GORM Postgres bootstrap (open, pool tuning, ping)
    ├── query/         list-endpoint standard: pagination, sort whitelist, Result envelope
    ├── validate/      struct validation with per-field messages
    ├── requestid/     request id header + context propagation
    ├── logger/        slog JSON logger + ContextHandler
    ├── telemetry/     OTel SDK bootstrap (TracerProvider, OTLP exporter, propagator)
    ├── httpclient/    outbound client: options (timeout, logging, tracing), Do + Response helpers
    ├── assert/        minimal test assertions
    └── docker/        start/stop containers for integration tests
```

**The dependency rule** — arrows always point inward:

```
adapter/rest ──► core/port ◄── core/service ──► core/domain
                     ▲
adapter/postgres ────┘
```

`core` never imports an adapter. `service` sees the database only through the
`port.UserRepository` interface; `rest` calls the core only through
`port.UserService`. `main.go` wires the concrete implementations together.
This is what makes the core testable with generated mocks (see
`core/service/user_test.go`, regenerate with `make mocks`) and the postgres
adapter testable in isolation against a real container (see
`adapter/postgres/user_test.go`).

**Request flow for `POST /api/v1/users`:**

```
web.Handle(rest.userHandler.create)
  create:                        decode + validate DTO, call in-port
  → service.UserService.Create   validate (business rule), call out-port
    → postgres.UserRepository    INSERT, map row → domain.User
  ← web.Response                 web.Respond encodes DTO / errorResponse maps
                                 domain errors to web.RespondError
web.Handle writes the pre-encoded response; unmapped errors → logged 500
```

Handlers never touch the ResponseWriter — they return a `web.Response` built
by `web.Respond`/`web.RespondError` (which encode and log eagerly), and
`web.Handle` writes it out (a light take on Ardan Labs' `foundation/web`).

**Each layer has its own model** — `rest.UserResponse` (json tags),
`domain.User` (no tags), `postgres.userRow` (GORM model) — so the API shape and
the schema can evolve independently of the business model.

**Middleware chain (the order is load-bearing, see `rest/router.go`):**

```
[CORS]  →  Tracing  →  RequestID  →  SecureHeaders  →  Log  →  Recover  →  MaxBytes  →  Timeout(30s)  →  handlers
```

CORS mounts only when `HTTP_CORS_ALLOWED_ORIGINS` is set. Per route group:
`/auth` gets a per-IP rate limit, `web.Auth` mounts on the protected group
only (so login, registration, probes and swagger stay public), and admin-only
routes sit in a nested group with `web.Authorize` on top of it.

1. `Tracing` (otelchi) outermost: extracts the inbound `traceparent` or starts
   a new trace, so the span exists in ctx for everything downstream —
   including the access log's `trace_id`.
2. `RequestID` next, so everything after it logs with the request id.
3. `Log` *outside* `Timeout`: `http.TimeoutHandler` buffers the inner
   handler's writes and discards them on timeout — placed outside, the access
   log records the **real** status the client received (503 on timeout).
4. `Timeout` cancels the request context at the deadline, aborting downstream
   work (pgx queries, outbound calls) — which is why every layer takes
   `ctx context.Context` as its first parameter.

## Making it yours

**Rename the module and service** (first thing after cloning):

```bash
scripts/rename.sh github.com/you/my-service
```

**List endpoints** follow one convention: `?page=&limit=` (pagination, 1-based),
`?sort=name,-created_at` (comma-separated, minus = descending, whitelisted per handler, unique tiebreaker appended for stable pagination), domain-specific
filters (e.g. `?name=`), and the `query.Result` envelope
`{items, total, page, limit}` — see `platform/query` and the users list
handler. For rich filter structs, `web.DecodeQuery[T]` maps query parameters
by `form` tags with the same validation as the body.

**Add an entity** (package per technology, file per entity):

1. `core/domain/product.go` — entity + domain errors
2. `core/port/product.go` — `ProductService` (in) + `ProductRepository` (out) interfaces;
   add them to `make mocks` and regenerate
3. `core/service/product.go` — business logic (+ `product_test.go` with mocked ports)
4. `adapter/postgres/product.go` — GORM repository implementation (+ row model in `model.go`)
5. `adapter/rest/product.go` + DTOs in `dto.go` (`validate` tags + a
   `Validate()` method) — handlers with the `web.Handler` signature: return
   `web.Respond(ctx, status, body)` or `web.RespondError(ctx, status, msg, cause...)`,
   read route params via `r.PathValue`; mount with `web.Handle` in `router.go`
6. wire it in `cmd/api/main.go`

**Call an external API:** define an out-port in `core/port` (e.g.
`RateProvider`), implement it in a new driven adapter named after the system
(`adapter/hnb/`, `adapter/stripe/`) using `platform/httpclient`, inject it
into the service via `main.go`. The core never knows HTTP is involved, tests
mock the port, and the call automatically shows up as a client span in the
trace with `traceparent` forwarded downstream.

**Roles:** registration always creates a `user`; listing users requires `admin`, and get/update/
delete allow admins or the account owner. Roles are not changeable through the API — promote the
first admin manually: `UPDATE users SET role = 'admin' WHERE email = '...';`

**Add a migration:** `make migrate-create name=create_products`, edit the SQL in
`migrations/` — it ships inside the binary via
`embed.FS` and runs on boot (`DB_MIGRATE=true`).

## Configuration

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `dev` | environment name |
| `DEBUG` | `true` | debug log level when true |
| `HTTP_ADDR` | `:8080` | listen address |
| `HTTP_READ_TIMEOUT` | `10s` | max time to read the request |
| `HTTP_WRITE_TIMEOUT` | `35s` | max time to write the response; **must be > `HTTP_REQUEST_TIMEOUT`** (validated at boot) |
| `HTTP_IDLE_TIMEOUT` | `120s` | keep-alive idle timeout |
| `HTTP_REQUEST_TIMEOUT` | `30s` | per-request deadline (`http.TimeoutHandler`) |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | graceful shutdown drain window |
| `HTTP_MAX_BODY_BYTES` | `1048576` | max request body size (413 when exceeded) |
| `HTTP_CORS_ALLOWED_ORIGINS` | — | comma-separated origins for CORS; empty disables CORS |
| `AUTH_SECRET` | — (**required**) | JWT HS256 signing secret |
| `AUTH_TOKEN_TTL` | `15m` | access token lifetime |
| `DB_DSN` | — (**required**) | Postgres DSN |
| `DB_MIGRATE` | `true` | apply embedded migrations on boot |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` | `10` / `5` | sql.DB pool size bounds |
| `DB_CONN_MAX_LIFETIME` / `DB_CONN_MAX_IDLE_TIME` | `1h` / `30m` | pool connection recycling |
| `DB_SLOW_QUERY_THRESHOLD` | `200ms` | queries slower than this are logged as warnings |
| `OTEL_ENABLED` | `false` | export traces via OTLP; when off, spans are created locally (logs keep `trace_id`) but nothing is exported |
| `OTEL_SERVICE_NAME` | `anvil` | OTel `service.name` resource attribute |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC Collector, `host:port` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | plaintext gRPC (local dev only) |
| `OTEL_SAMPLE_RATIO` | `1.0` | fraction of traces to sample |

## Production notes & gotchas

- **Outbound HTTP calls**: the zero-value `http.Client` (and `http.DefaultClient`) has **no
  timeout** — use `platform/httpclient.New(...)`, which defaults to 30s and supports
  `WithTracing`/`WithLogging`. If you must build a custom transport, never start from
  `&http.Transport{}` (it drops proxy support, dial/TLS timeouts, HTTP/2 and pooling) — clone
  `http.DefaultTransport`. Never ship `InsecureSkipVerify: true`.
- **`IdleTimeout`**: when 0, net/http falls back to `ReadTimeout` for keep-alive connections;
  when neither is set, idle connections stay open until the client closes them. Always set it.
- **`http.TimeoutHandler`**: buffers the response and strips `http.Flusher`/`http.Hijacker` —
  mount streaming/SSE/websocket routes on a chi group **without** the `Timeout` middleware.
- **Graceful shutdown**: `srv.Shutdown` needs a *fresh* `context.Background()`-derived deadline —
  the signal context is already cancelled by the time you get there.
- **Context propagation**: `ctx` is the first parameter of every method, adapter → service →
  repository. This is what makes request timeouts actually cancel database queries. Enforced by
  the `noctx` and `contextcheck` linters.
- **Context keys**: always unexported named types (see `platform/requestid`), never
  string literals — collision-proof.
- **Router-agnostic handlers**: route params are read via the stdlib `r.PathValue` (chi
  populates it since v5.1), and all middleware is `func(http.Handler) http.Handler` — swapping
  the router touches only `adapter/rest/router.go`.
- **Logging**: use the `*Context` variants (`slog.InfoContext(ctx, ...)`) everywhere — plain
  `slog.Info` skips the ContextHandler enrichment and loses `request_id`.
- **Primary keys**: don't use random UUIDs (v4) — they fragment btree indexes. Use
  `bigint GENERATED ALWAYS AS IDENTITY` (see the users migration) or UUIDv7 if you need globally
  unique IDs; encode/encrypt IDs at the API boundary instead.
- **Goroutine limits**: bound fan-out with `x/sync/errgroup` (`SetLimit`) and gate hot sections
  with `x/sync/semaphore` — never spawn unbounded goroutines per item.
- **GORM + goose**: both run on pgx (GORM via its postgres driver, goose via pgx's `stdlib`
  driver) — don't add a second Postgres driver (lib/pq).
- **Rate limiting**: the login limit keys off `r.RemoteAddr` — behind a reverse proxy that is the
  proxy's address, so key off a trusted client-IP header there instead.
- **CSRF**: not applicable while auth stays in the `Authorization` header — browsers never attach
  it automatically. If you move the JWT into a cookie, you take on CSRF: set `SameSite`, and add
  Go's `http.CrossOriginProtection` middleware.
- **`platform/assert`** is the zero-dependency assertion helper used across the tests.
