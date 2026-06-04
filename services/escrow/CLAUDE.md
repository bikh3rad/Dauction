# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Escrow service (Dauction) — "the heart"

Bounded context: **the funds ledger + escrow state machine** (root `CLAUDE.md` §4). Escrow is the
**sole writer** of escrow rows and enforces the funds-conservation invariant. Owns its `escrow`
Postgres DB and integrates only through events (NATS/JetStream) and each owner's sync API — never
DB→DB (root §0–§2). All amounts are `int64` USDC cents.

**Tables (owned, `migrations/`):**
- `escrow_trade` — head record (`id` = trade_id = auction_id); `lot_id`, `buyer_account_id` (winner),
  `seller_account_id`, `kind` (`DUTCH|PASSIVE`, CHECK), `state`
  (`UNLOCKED|DEPOSIT_LOCKED|FULL_LOCKED|HELD|RELEASED|REFUNDED|FORFEITED|DISPUTED`, CHECK),
  `price_cents`, `premium_cents`, `fee_cents`, `inspector_fee_cents` (BIGINT), `release_mode`
  (`CASH|VAULT_CREDIT`, nullable CHECK), `funding_deadline` (now+24h on passive win), timestamps.
- `escrow_ledger` — **APPEND-ONLY** (never updated/deleted); `id`, `trade_id`, `participant_account_id`,
  `entry_type` (`DEPOSIT_LOCK|FULL_LOCK|HOLD|RELEASE|REFUND|FORFEIT|FEE|PREMIUM|INSPECTOR_FEE`, CHECK),
  `amount_cents` (signed BIGINT — refunds negative, all else positive), `ref`, `created_at`. Derived
  per-(trade, participant) balance = `SUM(amount_cents)`. Index `(trade_id, created_at)` and
  `(trade_id, participant_account_id)`.
- `outbox` — transactional outbox (UNIQUE `idempotency_key`, partial index on unpublished).
- `consumed_event` — inbox/idempotency ledger (PK `idempotency_key`).

**State machine (enforced in `biz`; illegal → `ErrResourceInvalid`):**
`UNLOCKED → DEPOSIT_LOCKED → FULL_LOCKED → HELD → RELEASED`; branches `→ REFUNDED` (loser/dispute),
`→ FORFEITED` (winner missed 24h funding), `HELD → DISPUTED → {RELEASED|REFUNDED}` by ruling.
Dutch: reserve(10%) → full-lock(100%) → hammer → HELD. Passive: `auction.won` creates UNLOCKED with
funding_deadline; the winner funds price+premium straight into HELD. Transitions are atomic
conditional UPDATEs (`WHERE id=$ AND state=from`); a non-matching row → invalid.

**THE conservation invariant (root §4; enforced in `biz`, asserted in tests):** once funds are locked,
the gross **inflows** `{DEPOSIT_LOCK, FULL_LOCK, HOLD}` are constant; the gross **disbursements**
`{RELEASE, REFUND(abs), FORFEIT, FEE, PREMIUM, INSPECTOR_FEE}` never exceed inflows and equal them at
settlement (`entity.SummariseConservation` / `Conservation.Balanced`). Every transition writes
balancing rows; a transition that would break conservation (e.g. carve-outs exceeding the pot) is
rejected before any write. A seeded property/fuzz test (`conservation_fuzz_test.go`) drives random
valid Dutch+passive sequences (refunds, forfeits, all three rulings) and asserts the invariant after
every transition.

**Release math:** the held pot is carved into `RELEASE` (to seller) + `FEE` + `PREMIUM` +
`INSPECTOR_FEE`; `release = pot − fee − premium − inspector` (no money minted). `CASH` releases 100%;
`VAULT_CREDIT` records a **110%** credit instruction on the event (`ReleaseCreditCents`, integer
truncation) — the extra 10% is vault's loyalty top-up, off the escrow ledger. **SPLIT** halves the
pot; the **odd cent goes to the buyer** (`seller = pot/2`, `buyer = pot − pot/2`).

**Routes** (handlers self-register, Go 1.22 patterns, swag-annotated; caller from `X-Account-Id`):
- `GET  /apis/escrow/{tradeId}` — trade state + derived per-participant balances + conservation.
- `POST /apis/escrow/{tradeId}/fund` `{amountCents}` — winner funds; amount must equal obligation
  exactly; past deadline → FORFEITED; double-fund rejected.
- `POST /apis/escrow/{tradeId}/confirm` `{mode: CASH|VAULT_CREDIT}` — buyer confirms delivery →
  RELEASED; emits `escrow.released`; blocked while DISPUTED.
- `POST /apis/admin/escrow/{tradeId}/refund` `{participantId}` — loser/manual refund → REFUNDED.
- `POST /apis/admin/escrow/{tradeId}/forfeit` — manual forfeit → FORFEITED.
- **Locking is event-driven, not HTTP:** reservation/full-lock arrive as `escrow.lock_requested`
  (escrow is the sole funds authority; the auction engines request locks via the bus, root §2).

**Events emitted (outbox → NATS):** `escrow.locked` `{trade_id, participant_id, state, amount}`
(DEPOSIT_LOCKED/FULL_LOCKED/HELD), `escrow.released` `{trade_id, seller_id, amount, as_vault_credit,
credit_cents}`, `escrow.forfeited`, `escrow.refunded`. Each commits in the SAME tx as the
state/ledger write.

**Events consumed (inbox-idempotent):** `escrow.lock_requested` (Dutch reserve/full-lock: create-or-
advance trade + lock ledger row + emit `escrow.locked`), `auction.hammer` (Dutch: FULL_LOCKED → HELD),
`auction.won` (passive: create UNLOCKED + funding_deadline = now+24h), `dispute.resolved` (apply
REFUND_BUYER / RELEASE_SELLER / SPLIT). Replays/out-of-order deliveries are no-op successes (inbox +
conditional transitions).

**Deviations from `proto/`:** (1) `escrow.lock_requested` is not in the frozen proto oneof — it is
escrow's internal lock contract (JSON: `trade_id|auction_id, lot_id, buyer_id, seller_id, state,
amount`). (2) `escrow.refunded` has no proto arm (§2 lists it as emitted); shape mirrors the
locked/forfeited family. (3) `EscrowReleased` carries an extra JSON `credit_cents` (proto is
narrower) so vault credits the 110% without re-deriving it. (4) `auction.won` is read with an extra
`seller_id` JSON field the producer stamps (proto `AuctionWon` omits the seller), so escrow records
the release beneficiary without a DB→DB lookup.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `escrow`, host
`pg-escrow:5432`), `APP_DATASOURCE_NATS_DSN` (`nats:4222`, stream `DAUCTION`, durable `escrow`),
`APP_SERVER_HTTP_ADDR` (`:8080`), `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT` (`jaeger:4317`). Defaults in
`config.example.yaml`.

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
