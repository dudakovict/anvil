# CLAUDE.md

Production-ready Go backend starter, hexagonal architecture. The README covers
the architecture, request flow and configuration in detail — read it first.

## Commands

- `make test` — all tests with `-race`; the postgres integration test spins up
  its own docker container and skips itself under `-short` or without docker
- `go test -run TestName ./internal/core/service/` — single test
- `make lint` / `make fmt` — golangci-lint (wsl_v5 and nlreturn enforce blank
  lines; run fmt before lint)
- `make audit` — everything CI checks: tidy diff, verify, tests, lint, govulncheck
- `make mocks` — regenerate gomock mocks after changing `core/port` interfaces
- `make swagger` — regenerate `docs/` after changing handler annotations

## Skills

Load the matching skill before starting: `add-entity` (new domain entity
through all layers), `add-adapter` (external API behind an out-port),
`go-tests` (any test writing), `e2e-check` (full compose verification).

## Architecture rules

- Dependencies point inward: `adapter/*` → `core/port` ← `core/service` →
  `core/domain`; `core` never imports adapters; `platform/*` is generic infra
  that knows nothing about domains
- New entity = file per layer (domain, port, service, postgres, rest) — follow
  the `user` files as the template
- Persistence is GORM + goose SQL migrations (never AutoMigrate, never propose
  switching to direct pgx); domain errors map in `postgres` (gorm errors) and
  `rest/user.go:errorResponse` (HTTP statuses)
- Every function takes `ctx context.Context` first; log only via
  `slog.XxxContext(ctx, ...)`

## Style

- Comments: package doc (`// Package x provides support for ...`, in `doc.go`
  for multi-file packages) plus non-obvious contracts only — no obvious
  comments, no semicolons or em-dashes in comments
- `var` error blocks at the top of the file; struct literals one field per line
- Reuse the outer `err` (`if err = ...`) instead of shadowing (govet shadow is
  on); use `errors.AsType` over `errors.As`
- Tests: table-driven even for one case, fields `name` + `tests`/`tt` loop,
  external test packages (`package x_test`), `platform/assert`
  (`assert.Equal(t, got, want)`), gomock repos via per-case
  `mocks func(t) port.X` closures, John Doe fixtures — full skeletons in the
  `go-tests` skill, load it before writing any test
- List endpoints: `?page`/`?limit` (1-based, errors not clamping), `?sort`
  whitelist per handler, `query.Result` envelope
- Unexported by default; no getters with `Get` prefix; YAGNI — no abstractions
  before a second use exists
