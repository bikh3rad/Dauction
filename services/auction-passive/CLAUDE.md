# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Auction-Passive service (Dauction)

Bounded context: **the timed, passive auctions — Vickrey (sealed second-price) and UniqBid
(lowest unique price)**: sealed bid intake, the immutable bid log, and the deterministic close &
resolution. This service owns its `auction_passive` Postgres DB and reaches other services only via
events/their APIs — never DB→DB (root `CLAUDE.md` §0–§2). The template plumbing docs below still
apply (Wire, koanf, OTel).

**Tables (owned, `migrations/`):**
- `auction` — `id` (UUID PK), `lot_id` (UNIQUE; from catalog's `lot.scheduled`), `atype`
  (`VICKREY|UNIQBID`, CHECK), `state` (the passive machine, CHECK), `closes_at`, `reserve_cents`
  (BIGINT USDC cents), `winner_account_id` (NULL until resolved), `cleared_price_cents`,
  `created_at`. Index `(state, closes_at)` for the close sweep.
- `passive_bid` — the **IMMUTABLE** append-only bid log: `id`, `auction_id` (FK), `bidder_account_id`,
  `price_cents` (BIGINT, CHECK > 0), `placed_at` (server clock — the resolution tiebreaker),
  `debit_idempotency_key`, `created_at`. Index `(auction_id, placed_at)`. UNIQUE
  `(auction_id, bidder_account_id, price_cents)` (UniqBid distinct-price rule; Vickrey one-bid is
  also enforced in biz) and UNIQUE `(debit_idempotency_key)`.
- `outbox` — transactional outbox; `idempotency_key` UNIQUE; partial index on unpublished rows.
- `consumed_event` — inbox/idempotency ledger (PK `idempotency_key`).

**State machine (CLAUDE.md §3, enforced in `biz`, illegal → `ErrResourceInvalid`):**
`DRAFT → APPRAISING → SCHEDULED → OPEN → CLOSING → RESOLVED → SETTLING → COMPLETED`, with `ABORTED`
for a UniqBid that resolves to no unique price. Auctions are created **OPEN** from `lot.scheduled`
(`closes_at = scheduled_at + duration_days`). Bids are accepted only while `OPEN ∧ now < closes_at`.

**Resolution (the core — a PURE function of the immutable log + `placed_at`, `internal/biz/resolution.go`):**
- `ResolveVickrey(bids) (Result, error)` — winner = bidder of the **2nd-highest DISTINCT price**, pays
  that price; ties on a price → earliest `placed_at` owns it; one distinct price (single bidder / all
  equal) → that bidder wins at their own price.
- `ResolveUniqBid(bids) (Result, error)` — among prices chosen by exactly one participant, the
  **minimum** wins at that price; no unique price → `Won=false` (auction ABORTS).
- Table-driven + seeded-random **property/fuzz** tests assert the winner rules (ties, single-bidder,
  no-unique) and determinism (`resolution_test.go`).

**Routes** (handlers self-register, Go 1.22 method patterns, swag-annotated; caller from `X-Account-Id`):
- `GET  /apis/auctions/{id}` — **public** auction info (state, closes_at, participant count). Never
  reveals other bidders' prices.
- `POST /apis/auctions/{id}/bid` — `{ priceCents, requestId }`. The bid-credit flow (CLAUDE.md §5):
  validate OPEN+window+price → **debit 1 credit synchronously** via the bids service → persist the
  immutable bid + `bid.placed` outbox event in one tx. Out of credits / closed → `RESOURCE_INVALID`.
- `GET  /apis/auctions/{id}/standing` — the caller's own sealed view (VICKREY: your sealed bid;
  UNIQBID: your prices + `isLowestUnique` per price, server-computed).
- `POST /apis/admin/auctions/{id}/close` — OPEN→CLOSING→RESOLVED/ABORTED; runs resolution; emits
  `auction.closed` then `auction.won`.

**The bid-credit rule (CLAUDE.md §5 — the key integration):** placing a passive bid costs exactly 1
bid credit. `biz.CreditDebitor` (implemented in `repo/creditdebitor.go` as an HTTP client to
`{bids.baseUrl}/apis/internal/bids/debit`) is called **before** the bid is persisted. The debit
idempotency key is derived deterministically from `(auction, bidder, price, requestId)`, so a retried
bid-write (same key) replays the debit as a **no-op** at bids — a credit is never burned without a
recorded bid, and never double-burned.

**Events emitted** (via the **outbox**, in the same tx as the state change):
- `bid.placed` {auction_id, bidder_id, bid_id, amount, placed_at}
- `auction.closed` {auction_id, lot_id, mode, closed_at}
- `auction.won` {auction_id, lot_id, winner_id, cleared_price, premium}

**Events consumed** (idempotent via the `consumed_event` inbox, subject-scoped key):
- `lot.scheduled` (from catalog) — VICKREY/UNIQBID only (DUTCH ignored) → create an **OPEN** auction.
- `bids.debited` (from bids) — **reconciliation only**; the synchronous debit call is authoritative.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `auction_passive`, host
`pg-auction-passive`), `APP_DATASOURCE_NATS_DSN`, `APP_BIDS_BASEURL` (default `http://bids:8080`),
`APP_SERVER_HTTP_ADDR` (default `:8080`), `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT`. Defaults in
`config.example.yaml` target `deploy/docker-compose.yml`. NATS stream `DAUCTION`, durable
`auction-passive`.

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
