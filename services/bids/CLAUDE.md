# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Bids service (Dauction)

Bounded context: **the bid-credit wallet, credit packages, purchase, and idempotent debit-on-bid**
(root `CLAUDE.md` §5). This service owns its `bids` Postgres DB and reaches other services only via
events/their APIs — never DB→DB (root §0–§2). It is **called synchronously** by `auction-passive`
(it consumes no events). The template plumbing docs below still apply (Wire, koanf, OTel).

**The unit rule (root §0.4, §5):** `bid_credit` is `int64` WHOLE credits ($1 each); USDC amounts are
`int64` cents. The two units NEVER share a field or column.

**Tables (owned, `migrations/`):**
- `bid_package` — `id` (PK, e.g. `PKG_100`), `credits` (BIGINT, CHECK > 0), `price_cents` (BIGINT
  USDC cents, CHECK > 0), `best_value`. Seeded: `{PKG_100,100,$80}`, `{PKG_50,50,$45}`, `{PKG_20,20,$20}`.
- `bid_wallet` — `account_id` (PK), `balance_credits` (BIGINT, CHECK ≥ 0), `updated_at`. Read-through;
  never recomputed in a handler.
- `bid_purchase` — `id`, `account_id`, `package_id` (FK), `credits_granted` (BIGINT), `usdc_charged_cents`
  (BIGINT), `created_at`. The USDC charge and the credit grant commit atomically in one tx.
- `bid_debit` — `id`, `account_id`, `amount_credits` (BIGINT), `idempotency_key` (UNIQUE), `auction_id`,
  `created_at`. Idempotent: a replay with the same key returns the original debit and burns nothing.
- `outbox` — transactional outbox (`idempotency_key` UNIQUE, partial index on unpublished rows).

**Routes** (handlers self-register, Go 1.22 patterns, swag-annotated; caller from `X-Account-Id`):
- `GET  /apis/bids/wallet` — caller's balance + recent purchases/debits.
- `GET  /apis/bids/packages` — PUBLIC package catalogue.
- `POST /apis/bids/buy` `{packageId, idempotencyKey?}` — records the purchase atomically, emits
  `bids.purchased`. Unknown package → `RESOURCE_INVALID`.
- `POST /apis/internal/bids/debit` `{accountId, amount, idempotencyKey, auctionId}` — the sync endpoint
  `auction-passive` calls BEFORE recording a bid. Insufficient balance → `RESOURCE_INVALID`
  ("out of credits"). Idempotent on `idempotencyKey`: a replay returns the original debit (HTTP 200,
  same body), never double-burns. Emits `bids.debited`.

**Concurrency rule (root §5):** debit is safe under concurrent calls — a UNIQUE `bid_debit` insert
(`ON CONFLICT DO NOTHING`, the idempotency gate) plus a conditional `UPDATE bid_wallet SET balance =
balance - $n WHERE account_id = $1 AND balance_credits >= $n`, all in ONE tx. The conditional UPDATE
matching no row ⇒ insufficient funds ⇒ `ErrResourceInvalid` (the claimed debit row rolls back).

**Events emitted (outbox → NATS, same tx as the state change):**
- `bids.purchased` `{account_id, package_id, credits_granted, usdc_charged}`
- `bids.debited` `{account_id, amount, idempotency_key, balance}` — the outbox row is built INSIDE the
  debit tx (via `biz.NewDebitedOutbox`) once the post-debit balance is known, and emitted only on a
  fresh burn (a replay emits nothing). **Events consumed:** none (called synchronously).

**Errors:** language-neutral machine codes (`RESOURCE_NOT_FOUND|RESOURCE_INVALID|RESOURCE_EXISTS|
ACCESS_DENIED|INTERNAL`) in the response `code` field; biz sentinels mapped via `dto.HandleError`.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_POSTGRES_DSN` (db `bids`, host `pg-bids:5432`),
`APP_DATASOURCE_NATS_DSN` (`nats:4222`, stream `DAUCTION`, durable `bids`, subjects
`bids.purchased,bids.debited`), `APP_SERVER_HTTP_ADDR` (`:8080`),
`APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT` (`jaeger:4317`). Defaults in `config.example.yaml`.

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
