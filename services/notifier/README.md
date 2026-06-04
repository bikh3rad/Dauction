# Dauction — Notifier service

The **realtime fan-out edge**. The notifier subscribes to domain events on NATS/JetStream and
broadcasts **server-computed view-state** to connected clients over **Server-Sent Events (SSE)**:
the live descending Dutch price, passive auction countdowns/standings, hammer & resolution
outcomes, escrow state changes, and activity toasts.

It owns **no domain database** and makes **no authority decisions** — the socket is a strictly
read-only view (root `CLAUDE.md` §6). All payloads are language-neutral (enum state codes, integer
USDC cents, ISO-8601 UTC); the React client localizes. The Go module is named `application`.

## Transport: SSE (not WebSocket)

The feed is strictly server→client, so SSE is the natural fit and — critically — needs **no extra
dependency** (plain `net/http`). WebSocket was avoided because adding a WS library to `go.mod`
isn't viable in the build-restricted environment. SSE still meets the <100 ms broadcast target.

## What it does

- **Subscribe** (durable `notifier`) to every consumed subject and project each event into a
  broadcast `entity.Message` — never a raw event, never a sealed passive bid price.
- **Compute the Dutch price locally**: `current_price(now) = max(floor, ceiling − step·⌊(now−open)/interval⌋)`,
  re-implemented identically to `auction-dutch` from the params carried on `auction.opened`. A
  per-service ticker re-broadcasts price + `next_drop_at` every `notifier.tickInterval` (default 1s)
  for each OPEN Dutch auction.
- **Hold ephemeral subscriptions** in an in-memory `Hub`: rooms keyed `auctions/{id}` and
  `me/{accountId}`; concurrency-safe; **drop-oldest** per-client backpressure so a slow reader can
  never block the hub.
- **Snapshot on connect**: a small in-memory `Registry` of currently-open auctions lets a
  reconnecting client get an accurate first frame immediately.

## Routes

| Path | Notes |
|---|---|
| `GET /apis/live/auctions/{id}` | **SSE** — snapshot on connect, then the auction room's live feed |
| `GET /apis/live/me` | **SSE** — caller's personal feed (escrow state, toasts); identity from gateway-injected `X-Account-Id` |
| `GET /healthz/liveness` `GET /healthz/readiness` | health (fans out across registered checks incl. NATS) |
| `GET /metrics` | Prometheus exposition |
| `GET /swagger/` | Swagger UI (spec at `/docs/swagger/swagger.json`) |

## Consumed event → broadcast projection

| Subject | Broadcast kind | Room(s) | Fields (server-computed only) |
|---|---|---|---|
| `auction.opened` | `DUTCH_PRICE` | `auctions/{id}` | `currentPriceCents`, `nextDropAt`, `state=OPEN` (+ starts the ticker) |
| (ticker) | `DUTCH_PRICE` | `auctions/{id}` | recomputed price + `nextDropAt`, every `tickInterval` |
| `auction.hammer` | `HAMMER` | `auctions/{id}` | `winnerId`, `clearedPriceCents`, `state=HAMMER` (stops the ticker) |
| `auction.completed` | `COMPLETED` | `auctions/{id}` | `state=final_state`; room closed |
| `auction.closed` | `CLOSED` | `auctions/{id}` | `state=CLOSING`, `mode` |
| `auction.won` | `WON` | `auctions/{id}` | `winnerId`, `clearedPriceCents`, `state=RESOLVED` |
| `bid.placed` | `ACTIVITY` | `auctions/{id}` | `bidCount` toast **only** — the sealed price/bidder is NEVER broadcast |
| `escrow.locked` | `ESCROW_STATE` | `auctions/{id}` + `me/{participant}` | `state` (DEPOSIT_LOCKED/FULL_LOCKED/HELD), `amountCents` |
| `escrow.released` | `ESCROW_STATE` | `auctions/{id}` + `me/{seller}` | `state=RELEASED`, `amountCents` |
| `escrow.forfeited` | `ESCROW_STATE` | `auctions/{id}` + `me/{participant}` | `state=FORFEITED`, `amountCents` |
| `escrow.refunded` | `ESCROW_STATE` | `auctions/{id}` + `me/{participant}` | `state=REFUNDED`, `amountCents` |

Unknown subjects on the shared stream are acked as no-ops.

## Quickstart

```sh
cp config.example.yaml config.yaml
make generate
go run ./cmd/app --config ./config.yaml
```

Listens on `:8080`. Override any key via `APP_`-prefixed env (e.g. `APP_DATASOURCE_NATS_DSN`,
`APP_NOTIFIER_TICKINTERVAL=500ms`, `APP_SERVER_HTTP_ADDR=:9090`).

## Config (`config.example.yaml`)

- `server.http.addr` — `:8080`.
- `notifier.tickInterval` — Dutch price re-broadcast cadence (default `1s`).
- `datasource.nats` — `dsn`, `streamName: DAUCTION`, `durable: notifier`, and the `subjects`
  comma-list of every consumed event. **No Postgres** — the notifier owns no DB.
- `collector.exporters.otlp.endpoint` — OTLP gRPC (Jaeger `jaeger:4317`).

## Architecture

```
cmd/app                  main + wire composition root (wire_gen.go hand-written; toolchain blocked)
app/                     runtime infra (Application, HTTPServer, Controller, KConfig, Logger, OTLP)
internal/service         HTTP wiring (mux, /metrics, /swagger)
internal/service/handler SSE live handler + healthz
internal/biz             hub, registry, projector, price function, live subscription seam
internal/eventbus        runner: NATS subscription + Dutch price ticker (lifecycle hooks)
internal/datasource      NATS/JetStream consumer (no Postgres)
internal/entity          view-state structs (Message, PriceParams, OpenAuction)
internal/mocks           mockery mocks for internal/biz
```

Dependency direction: `cmd → app → service/handler → biz → datasource → entity`. The `eventbus`
runner sits above `biz` + `datasource` so neither imports the other.

## Tests

```sh
go test ./...
```

- `internal/biz/price_test.go` — mirrors auction-dutch's price table (floor clamp, boundaries,
  zero-elapsed) and asserts identical prices.
- `internal/biz/hub_test.go` — fan-out, unregister stops delivery, slow client never blocks (drop-oldest).
- `internal/biz/projector_test.go` — the full event→message projection table, incl. that sealed
  passive prices never appear in `bid.placed` broadcasts, me-room fan-out, and snapshots.
- `internal/service/handler/live_test.go` — httptest SSE server: snapshot-on-connect, 401 without
  `X-Account-Id`, and a post-connect broadcast reaching the client.

## Note on generated code

The Go toolchain is unavailable in this environment, so `cmd/app/wire_gen.go`, `internal/mocks/*`,
and `docs/*` were hand-written to the generators' output format. Run `make generate` / `make swagger`
to regenerate them once tooling is available.
