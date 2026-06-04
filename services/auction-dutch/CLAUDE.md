# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Auction-Dutch service (Dauction)

Bounded context: **the live descending-price (Dutch) auction engine** — reservation deposits, full
locks, the server-authoritative price, and the hammer. This service owns its `auction_dutch` Postgres
DB and reaches other services only via events/their APIs — never DB→DB (root `CLAUDE.md` §0–§4). The
template plumbing docs below still apply (Wire, koanf, OTel).

**Tables (owned, `migrations/`):**
- `auction` — `id` (UUID PK), `lot_id` (from catalog `lot.scheduled`, UNIQUE), `state`
  (`DRAFT|APPRAISING|SCHEDULED|OPEN|HAMMER|SETTLING|COMPLETED|CANCELLED|ABORTED`, CHECK),
  `ceiling_cents` / `floor_cents` / `drop_step_cents` (BIGINT USDC cents), `drop_interval_seconds`
  (BIGINT), `open_at` (price clock origin; NULL until OPEN), `hammer_at`, `winner_account_id`,
  `hammer_price_cents`, `created_at`. CHECK `floor ≤ ceiling`. Index `(state)`.
- `auction_participant` — PK `(auction_id, account_id)`; `kyc_approved`, `tier`
  (`GUEST|MEMBER|VIP`), `reservation_state` (10% deposit lock), `full_lock_state` (100% lock), each
  `REQUESTED|LOCKED|RELEASED`, CHECK. Index on the eligibility tuple.
- `reservation` — `id`, `auction_id` (FK), `account_id`, `kind` (`DEPOSIT_10|FULL_LOCK`, CHECK),
  `amount_cents` (BIGINT), `state` (`REQUESTED|LOCKED|RELEASED`, CHECK), `escrow_ref` (UNIQUE; the
  producer-stable idempotency key shared with escrow), `created_at`.
- `outbox` — transactional outbox; `idempotency_key` UNIQUE; partial index on unpublished rows.
- `consumed_event` — inbox/idempotency ledger (PK `idempotency_key`).

**Routes** (handlers self-register, Go 1.22 method patterns, swag-annotated):
- `GET  /apis/auctions/{id}` — **PUBLIC**: state + SERVER-computed `currentPriceCents` + `nextDropAt`.
- `POST /apis/auctions/{id}/reserve` — 10% reservation deposit request → REQUESTED reservation +
  `escrow.lock_requested` {kind: DEPOSIT_10}.
- `POST /apis/auctions/{id}/lock` — 100% full-lock request → `escrow.lock_requested` {kind: FULL_LOCK}.
- `POST /apis/auctions/{id}/buy` — **the hammer**: re-compute price server-side, validate OPEN ∧
  caller fully eligible, atomic OPEN→HAMMER (first valid buy wins). Emits `auction.hammer`.
- `POST /apis/admin/auctions/{id}/open` — SCHEDULED→OPEN (needs ≥1 fully-locked participant). Emits `auction.opened`.
- `POST /apis/admin/auctions/{id}/complete` — SETTLING→COMPLETED. Emits `auction.completed`.
- `POST /apis/admin/auctions/{id}/abort` — pre-settlement→ABORTED. Emits `auction.completed` {ABORTED}.

**Price (pure, server-authoritative, heavily tested — `biz/price.go`):**
`current_price(now) = max(floor, ceiling − drop_step·⌊(now−open_at)/interval⌋)`. The `Clock` is
injected (`biz.Clock`) so price + hammer are deterministic in tests. Clients render from params; the
buy decision is always re-validated server-side.

**Eligibility approach (chosen):** the gateway injects the caller's identity + eligibility as headers
`X-Account-Id`, `X-Account-Tier`, `X-Kyc-Approved` on reserve/lock; this service caches them on the
participant row. It does **not** consume `kyc.approved` / `account.tier_changed` itself. The two lock
states are advanced only by `escrow.locked`. Buying requires KYC ∧ tier ∈ {MEMBER,VIP} ∧ deposit
LOCKED ∧ full_lock LOCKED.

**Events emitted** (via the **outbox**, written in the same tx as the state change):
- `auction.opened` {auction_id, lot_id, ceiling, floor, drop_step, drop_interval_secs, opened_at}
- `auction.hammer` {auction_id, lot_id, winner_id, hammer_price, premium, hammered_at}
- `auction.completed` {auction_id, lot_id, final_state}
- `escrow.lock_requested` {auction_id, account_id, kind, escrow_state, amount, escrow_ref}
  — **DEVIATION:** this subject is **not** in the frozen `proto/dauction/events/v1/events.proto` oneof.
  It is auction-dutch's ASK to the escrow service to lock funds. The escrow service is expected to
  lock and then emit `escrow.locked` echoing `escrow_ref` (an additive field) so we can flip the
  matching reservation. Land it in the proto contract when escrow's owner agrees.

**Events consumed** (idempotent via the `consumed_event` inbox, subject-scoped key):
- `lot.scheduled` (from catalog) — **DUTCH only** (VICKREY/UNIQBID ignored) → create a SCHEDULED
  `auction` carrying lot params (reserve → floor; ceiling/step/interval seeded with sane defaults).
- `escrow.locked` (from escrow) → flip the reservation matched by `escrow_ref` REQUESTED→LOCKED and
  advance the participant's matching lock flag. Unknown `escrow_ref` → no-op.

**State rules (enforced in `biz`, illegal → `ErrResourceInvalid`):** `SCHEDULED → OPEN → HAMMER →
SETTLING → COMPLETED`, with `CANCELLED`/`ABORTED` terminal. Entry to OPEN requires ≥1 participant
with KYC ∧ tier ∧ both locks LOCKED. The hammer is a conditional UPDATE gated on `state = OPEN`
(first valid buy wins; later buys → `ErrResourceInvalid`). Enums are MONOSPACE_UPPERCASE strings
(the value string IS the wire code). Money is `int64` USDC cents.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `auction_dutch`, host
`pg-auction-dutch`), `APP_DATASOURCE_NATS_DSN`, `APP_SERVER_HTTP_ADDR` (default `:8080`),
`APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT`. Defaults in `config.example.yaml` target
`deploy/docker-compose.yml` (`pg-auction-dutch:5432`, `nats:4222`, `jaeger:4317`). NATS stream
`DAUCTION`, durable `auction-dutch`.

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
