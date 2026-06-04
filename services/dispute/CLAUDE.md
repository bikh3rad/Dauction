# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Dispute service (Dauction)

Bounded context: **the dispute court**. A buyer (claimant) raises a post-delivery
authenticity/condition/delivery claim on a trade; the house rules; escrow executes the
verdict. This service owns its `dispute` Postgres DB and reaches other services only via
events (root `CLAUDE.md` §0–§2, §4). The template plumbing below still applies (Wire, koanf, OTel).

**Tables (owned, `migrations/`):**
- `dispute` — `id` (UUID PK), `trade_id` (escrow trade / auction id), `claimant_account_id`,
  `respondent_account_id`, `reason_code` (`AUTHENTICITY|CONDITION|NOT_DELIVERED|OTHER`, CHECK),
  `state` (`OPEN|UNDER_REVIEW|RESOLVED|WITHDRAWN`, CHECK), `ruling`
  (`NULL` until RESOLVED, then `REFUND_BUYER|RELEASE_SELLER|SPLIT`, CHECK), `evidence_ref`,
  `ruled_by`, `created_at`, `resolved_at`. A partial UNIQUE index
  (`uq_dispute_open_per_trade` on `state IN ('OPEN','UNDER_REVIEW')`) enforces **one
  non-terminal dispute per trade**.
- `dispute_event` — IMMUTABLE audit trail (append-only, never updated/deleted): `id`,
  `dispute_id` (FK), `actor_account_id`, `action`
  (`OPENED|EVIDENCE_ADDED|REVIEW_STARTED|RULED|WITHDRAWN`, CHECK), `detail_ref`, `created_at`.
- `outbox` — transactional outbox (same shape as the platform): `id`, `subject`,
  `idempotency_key` (UNIQUE), `payload` (JSONB), `created_at`, `published_at`.
- `consumed_event` — inbox/idempotency ledger (provisioned for a future consumer; dispute
  is currently publish-only).

**Routes** (handlers self-register, Go 1.22 patterns, swag-annotated; caller from `X-Account-Id`):
- `POST /apis/escrow/{tradeId}/dispute` — buyer opens `{reasonCode, evidenceRef, respondent}`.
  Creates OPEN + OPENED audit row; emits `dispute.opened`.
- `GET  /apis/escrow/{tradeId}/dispute` — dispute + audit trail (claimant, respondent, or admin `X-Admin: true`).
- `POST /apis/escrow/{tradeId}/dispute/evidence` — either party appends `{detailRef}` (audit row).
- `POST /apis/escrow/{tradeId}/dispute/resolve` — admin rules `{ruling, ruledBy}` → RESOLVED; emits `dispute.resolved`.
- `POST /apis/escrow/{tradeId}/dispute/withdraw` — claimant retracts an OPEN dispute → WITHDRAWN.
- `GET  /apis/admin/disputes?state=` — admin queue.
- `POST /apis/admin/disputes/{id}/review` — admin moves OPEN → UNDER_REVIEW.

**Events emitted (via the outbox):**
- `dispute.opened` `{dispute_id, trade_id, claimant}` — **service-local** subject (NOT in the
  frozen `proto/dauction/events/v1/events.proto`, which only declares `DisputeResolved`).
  Escrow consumes it to suspend release (HELD → DISPUTED). Deviation documented in
  `internal/biz/events.go`.
- `dispute.resolved` `{dispute_id, trade_id, ruling}` — matches `dauction.events.v1.DisputeResolved`
  (the proto also carries optional SPLIT amounts; dispute emits only `ruling` and lets escrow
  compute the split). Escrow consumes it to execute the funds movement.

**Events consumed:** none. `escrow.released` auto-close of the eligibility window is intentionally
out of scope (kept simple); the `consumed_event` inbox + the no-op `biz.EventConsumer` are in
place so a consumer can be added without touching the eventbus runner.

**State rules (enforced in `biz`, illegal → `ErrResourceInvalid`):** lifecycle is
`OPEN → UNDER_REVIEW → RESOLVED` with `WITHDRAWN` as a terminal off-ramp from `OPEN`.
Only one non-terminal dispute may exist per trade (a second open → `ErrResourceExists`).
**Resolve requires UNDER_REVIEW** (a dispute is always triaged via `review` before a verdict;
resolving from OPEN is rejected). **Withdraw is claimant-only and OPEN-only.** The ruling is
**immutable** once set — repo transitions are CAS on the from-state, so a re-resolve fails with
`ErrResourceInvalid`. Reads are gated to parties (claimant/respondent) or admin. Every action
appends exactly one immutable `dispute_event` row, written in the same tx as the state change.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `dispute`),
`APP_DATASOURCE_NATS_DSN`, `APP_SERVER_HTTP_ADDR` (default `:8080`),
`APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT`. Defaults in `config.example.yaml` target
`deploy/docker-compose.yml` (`pg-dispute:5432`, `nats:4222`, `jaeger:4317`); NATS stream
`DAUCTION`, durable `dispute`, subjects `dispute.opened,dispute.resolved`.

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
