# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Notifier service (Dauction) — realtime fan-out edge

Bounded context: **the realtime view edge**. The notifier subscribes to domain events on
NATS/JetStream and broadcasts **server-computed view-state** to connected clients over
**Server-Sent Events (SSE)**. It owns **no domain DB and no Postgres** — only ephemeral in-memory
subscriptions — and makes **no authority decisions**: the socket is a strictly read-only view (root
`CLAUDE.md` §0, §2, §6). All payloads are language-neutral (enum state codes, integer USDC cents,
ISO-8601 UTC); the React client localizes.

**Transport — SSE, not WebSocket.** The feed is strictly server→client, and SSE needs no extra
dependency (plain `net/http`). A WS library could not be added to `go.mod` in the build-restricted
environment. SSE still meets the <100 ms target.

**Owns:** nothing persistent. In-memory only:
- **Hub** (`internal/biz/hub.go`) — rooms keyed `auctions/{id}` and `me/{accountId}`; clients
  register/unregister; concurrency-safe (RWMutex). **Backpressure = drop-oldest** per bounded
  per-client channel (buffer 16): a slow SSE reader never blocks the hub or other clients, and the
  newest price tick always survives.
- **Registry** (`internal/biz/registry.go`) — best-effort map of currently-OPEN auctions so a
  reconnecting client gets an accurate snapshot frame on connect. Rebuilt from the durable stream.

**Routes** (handlers self-register, Go 1.22 patterns, swag-annotated):
- `GET /apis/live/auctions/{id}` — **SSE**: snapshot on connect, then the auction room's live feed.
- `GET /apis/live/me` — **SSE**: caller's personal feed (escrow state, toasts); identity from the
  gateway-injected `X-Account-Id` header (401 if absent).

**Price (local, identical to auction-dutch — `internal/biz/price.go`):**
`current_price(now) = max(floor, ceiling − drop_step·⌊(now−open_at)/interval⌋)`. The notifier
re-implements the SAME pure function from the params carried on `auction.opened`, and a ticker
(`internal/eventbus/runner.go`, every `notifier.tickInterval`, default 1s) re-broadcasts price +
`next_drop_at` for each OPEN Dutch auction until hammer/complete. The `Clock` is injected for
deterministic tests.

**Events consumed → broadcast projection** (`internal/biz/projector.go`; the notifier **emits
nothing**):
- `auction.opened` → `DUTCH_PRICE` {currentPriceCents, nextDropAt}; registers the auction + arms the ticker.
- `auction.hammer` → `HAMMER` {winnerId, clearedPriceCents, state=HAMMER}; stops the ticker.
- `auction.completed` → `COMPLETED` {state=final_state}; room closed.
- `auction.closed` → `CLOSED` {state=CLOSING, mode}.
- `auction.won` → `WON` {winnerId, clearedPriceCents, state=RESOLVED}.
- `bid.placed` → `ACTIVITY` {bidCount} toast **only** — the sealed price/bidder is **NEVER** broadcast.
- `escrow.locked|released|forfeited|refunded` → `ESCROW_STATE` {state, amountCents} to the trade's
  `auctions/{trade_id}` room **and** the affected `me/{participant}` room.

Unknown subjects on the shared stream are acked as no-ops.

**Config env (koanf `APP_`-prefixed):** `APP_DATASOURCE_NATS_DSN` (`nats:4222`, stream `DAUCTION`,
durable `notifier`, subjects = the consumed list above), `APP_NOTIFIER_TICKINTERVAL` (default `1s`),
`APP_SERVER_HTTP_ADDR` (`:8080`), `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT` (`jaeger:4317`). **No
postgres block.** Dockerfile runtime image is public `gcr.io/distroless/static-debian12:nonroot`.

**Toolchain note.** The Go toolchain was blocked when this service was built, so
`cmd/app/wire_gen.go`, `internal/mocks/*`, and `docs/*` were hand-written to the generators' output
format. Run `make generate` / `make swagger` once tooling is available.

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
