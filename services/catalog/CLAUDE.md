# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Catalog service (Dauction)

Bounded context: **lots, the weekly 32-lot cap, the certification gate (inspector attestation), and
the public gallery reads**. This service owns its `catalog` Postgres DB and reaches other services
only via events/their APIs — never DB→DB (root `CLAUDE.md` §0–§2). The template plumbing docs below
still apply (Wire, koanf, OTel).

**Tables (owned, `migrations/`):**
- `lot` — `id` (UUID PK), `object_id` (UNIQUE; from vault's `object.listed`), `seller_account_id`,
  `title`, `description`, `atype` (`DUTCH|VICKREY|UNIQBID`, CHECK), `duration_days` (NULL for DUTCH,
  else 2/5/7, CHECK + per-mode CHECK), `reserve_cents` / `appraised_value_cents` (BIGINT USDC cents),
  `state` (`DRAFT|CERTIFIED|SCHEDULED|REJECTED`, CHECK), `iso_week` (e.g. `2026-W23`), `created_at`,
  `scheduled_at`. Indexes: unique `(object_id)`, and `(iso_week, state)` for the weekly gallery + cap.
- `attestation` — `id`, `lot_id` (FK), `inspector_id`, `result` (`PASS|FAIL`, CHECK), `notes_ref`,
  `recorded_at`. A **PASS** is the certification gate. Index `(lot_id, recorded_at)`.
- `outbox` — transactional outbox; `idempotency_key` UNIQUE; partial index on unpublished rows.
- `consumed_event` — inbox/idempotency ledger (PK `idempotency_key`).

**Routes** (handlers self-register, Go 1.22 method patterns, swag-annotated):
- `GET  /apis/gallery/weekly` — **PUBLIC**: this ISO week's SCHEDULED lots (+ supply cap 32). `?week=`.
- `GET  /apis/lots/{id}` — **PUBLIC**: lot detail (atype, params, state, attestation summary).
- `GET  /apis/admin/lots` — admin list/filter by `?state=` and/or `?week=`.
- `POST /apis/admin/lots/{id}/attest` — record an inspector attestation `{inspectorId, result, notesRef}`.
- `POST /apis/admin/lots/{id}/certify` — DRAFT→CERTIFIED (requires a PASS attestation).
- `POST /apis/admin/lots/{id}/schedule` — CERTIFIED→SCHEDULED `{scheduledAt?}` (enforces weekly 32-cap).

**Events emitted** (via the **outbox**, written in the same tx as the state change):
- `attestation.recorded` {attestation_id, lot_id, inspector, pass}
- `lot.certified` {lot_id, attestation_id}
- `lot.scheduled` {lot_id, object_id, mode (atype), duration_days, scheduled_at, reserve_cents, week}

**Events consumed** (idempotent via the `consumed_event` inbox, subject-scoped key):
- `object.listed` (from vault) → create a **DRAFT** `lot` carrying atype + durationDays + appraised value.

**State rules (enforced in `biz`, illegal → `ErrResourceInvalid`):** `DRAFT → CERTIFIED → SCHEDULED`
with `REJECTED` terminal. **Certification gate:** a lot cannot be CERTIFIED without an existing PASS
attestation. A FAIL attestation on a DRAFT lot rejects it. **Weekly 32-cap:** at most 32 lots may be
SCHEDULED per ISO week — enforced atomically in `repo.ScheduleTx` via a conditional UPDATE gated on a
same-week SCHEDULED count subquery (exceeding → `ErrResourceInvalid`). Enums are MONOSPACE_UPPERCASE
strings (the value string IS the wire code). Money is `int64` USDC cents.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `catalog`, host `pg-catalog`),
`APP_DATASOURCE_NATS_DSN`, `APP_SERVER_HTTP_ADDR` (default `:8080`),
`APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT`. Defaults in `config.example.yaml` target `deploy/docker-compose.yml`
(`pg-catalog:5432`, `nats:4222`, `jaeger:4317`). NATS stream `DAUCTION`, durable `catalog`.

---

## Module name

The Go module is `application` (not `go-template`). All internal imports use this prefix, e.g. `application/internal/biz`, `application/app`.

## Common commands

```sh
# Regenerate Wire DI graph + go.mod tidy. Default make target.
make generate

# Install dev tools (golangci-lint, gofumpt, mockgen, swag, gci) — run once.
make devtools

# Lint (uses many enabled linters; expects an issues.exclude.yaml that may not exist —
# if missing, drop --config or run `golangci-lint run` directly using .golangci.yaml).
make check

# Generate Swagger docs into ./docs/ (entry: internal/service/server.go).
make swagger

# Run the app locally (requires config.yaml — copy from config.example.yaml).
go run ./cmd/app --config ./config.yaml

# Build container.
make build   # → docker image `buildf`
```

### Tests

Run a single test:
```sh
go test ./internal/service/handler/... -run TestName -v
```

Note: the `unit_tests`, `bench_tests`, `coverage_tests`, and `all_tests` Make targets reference stale paths (`internal/v1/http/handler/...`, `internal/handler/...`) that do not exist in the current tree. The actual handler tests live under `internal/service/handler/`. Prefer `go test ./...` or target the real paths directly until the Makefile is fixed.

## Architecture

Clean-architecture layout wired together with **Google Wire** (compile-time DI). The dependency direction is `cmd → app → service/handler → biz → repo → datasource → entity`.

Composition root: `cmd/app/wire.go` (build tag `wireinject`) calls `wire.Build` with provider sets from each layer. `make generate` writes `cmd/app/wire_gen.go`. **Do not hand-edit `wire_gen.go`** — modify the relevant `wire.go` provider set in `app/`, `internal/biz/`, `internal/repo/`, `internal/datasource/`, `internal/service/`, or `internal/service/handler/` and regenerate.

### Layers

- **`app/`** — runtime infrastructure: `Application` lifecycle (`Start`/`Shutdown`), `HTTPServer` (with otelhttp + panic recovery), `Controller` registry, `KConfig` (koanf), `AppLogger` (slog + OTEL log bridge), `OTLP` (traces/metrics/logs exporters). All providers are aggregated in `app.AppProviderSet`.
- **`internal/service/`** — HTTP wiring. `NewHTTPHandler` accepts a variadic `[]service.Handler` (each handler self-registers routes onto the shared `*http.ServeMux`) and additionally mounts `/metrics` (Prometheus) and `/swagger/`.
- **`internal/service/handler/`** — concrete HTTP handlers. To add one: implement `service.Handler` (`RegisterHandler(ctx) error` registers routes on the injected `*http.ServeMux`), add a `New…` provider, and append it to `NewServiceList` in `handler/wire.go` so it joins `[]service.Handler`. Apply middlewares via `pkg/middlewares.MultipleMiddleware`.
- **`internal/biz/`** — use cases. Interfaces named `Usecase…` are consumed by handlers; `Repository…` interfaces are implemented by `internal/repo/` (this is the seam mocked in tests).
- **`internal/repo/`** — repository implementations bound to use-case `Repository…` interfaces via `wire.Bind` in `repo/wire.go`.
- **`internal/datasource/`** — DB/queue clients (Postgres via pgx + otelsql, Redis, NATS, in-memory). Each datasource registers its own healthz + shutdown hooks on the shared `Controller` (see `PostgresDB.NewPostgresDB`).
- **`internal/entity/`** — plain domain structs.
- **`internal/mocks/`** — mockery-generated mocks for the `internal/biz` package; configured in `.mockery.yaml` (run via `go generate`).

### Cross-cutting patterns

- **Lifecycle via `app.Controller`**: components self-register `RegisterStartup`, `RegisterShutdown`, and `RegisterHealthz(name, fn, opts…)` rather than being orchestrated centrally. Healthz options control liveness vs. readiness participation; the `biz.healthz` use case fans out concurrently across the registered checks.
- **Configuration**: `KConfig` is loaded from a YAML file (`--config` flag, default `./config.yaml`) and overlaid with env vars prefixed `APP_` (e.g. `APP_SERVER_HTTP_ADDR` → `server.http.addr`). Each component has its own `New…Config` constructor that unmarshals a sub-tree.
- **Observability**: traces/metrics/logs go through OTLP gRPC. New code should obtain tracers via `otel.Tracer(...)` and use the slog logger passed through DI; the slog handler bridges to OTEL logs automatically.
- **HTTP middlewares**: there are two recovery middlewares (`app.NewRecoveryMiddleware` wrapping the whole server, and `pkg/middlewares.NewRecoveryMiddleware` for per-route use). Compose per-route middlewares with `middlewares.MultipleMiddleware(handler, mws...)`.

## Config bootstrap

`config.yaml` is required at runtime (the binary's default `--config` path is `./config.yaml`). Start from `config.example.yaml`:
```sh
cp config.example.yaml config.yaml
```
The Dockerfile bakes `config.example.yaml` in as `/app/config.yaml`.

## Local infra

`docker-compose.yml` aggregates includes from `infra/compose/` (monitoring stack, Redis, Postgres). `Tiltfile` references a Helm chart at `./charts/live-epg` that is **not** present in this repo — Tilt usage requires that chart to be added separately.
