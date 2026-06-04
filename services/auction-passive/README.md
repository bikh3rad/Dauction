# Dauction — Auction-Passive service

The **timed, passive auctions** for Dauction: **Vickrey** (sealed second-price) and **UniqBid**
(lowest unique price). Owns sealed bid intake, the immutable bid log, and the deterministic close &
resolution. Built from the `mequq/go-template` clean-architecture template (Go module `application`).

## Responsibilities

- Create an **OPEN** passive auction when catalog schedules a VICKREY/UNIQBID lot (`lot.scheduled`,
  `closes_at = scheduled_at + duration_days`).
- Accept **sealed bids** while the auction is OPEN and within its window. Each accepted bid spends
  exactly **one bid credit** (debited synchronously from the `bids` service **before** the bid is
  persisted; see the credit rule below).
- Store bids in an **immutable, append-only log** (`passive_bid`) keyed by server `placed_at`.
- On close, run a **pure, deterministic resolution** of the log and emit the winner.

## Resolution (the core)

`internal/biz/resolution.go` — pure functions, no I/O, fully table-/fuzz-tested:

- **Vickrey** — winner is the bidder of the **2nd-highest distinct price** and pays that price; ties
  on a price → earliest `placed_at`; a single distinct price → that bidder wins at their own price.
- **UniqBid** — among prices chosen by exactly one participant, the **minimum** wins at that price;
  no unique price → the auction **ABORTS**.

## The bid-credit rule (CLAUDE.md §5)

`POST /apis/auctions/{id}/bid` calls `biz.CreditDebitor.Debit` (an HTTP client to the `bids` service)
**before** persisting the bid. The debit idempotency key is derived deterministically from
`(auction, bidder, price, requestId)`, so a retried bid-write replays the debit as a **no-op** at
bids: a credit is never burned without a recorded bid, and never double-burned.

## HTTP surface (mounted under `/apis` by the gateway)

| Method & path | Purpose |
|---|---|
| `GET  /apis/auctions/{id}` | Public auction info (state, closes_at, participant count). |
| `POST /apis/auctions/{id}/bid` | Place a sealed bid (`{priceCents, requestId}`); spends 1 credit. |
| `GET  /apis/auctions/{id}/standing` | The caller's own sealed view (UNIQBID: prices + `isLowestUnique`). |
| `POST /apis/admin/auctions/{id}/close` | Close & resolve: emits `auction.closed` then `auction.won`. |

Caller identity comes from the `X-Account-Id` header (gateway-injected). Errors are language-neutral
codes via `dto.HandleError`.

## Events

- **Emits** (outbox): `bid.placed`, `auction.closed`, `auction.won`.
- **Consumes** (inbox, idempotent): `lot.scheduled` (VICKREY/UNIQBID → create auction; DUTCH ignored),
  `bids.debited` (reconciliation only).

## Data

Owns the `auction_passive` Postgres DB (`migrations/`): `auction`, `passive_bid` (immutable),
`outbox`, `consumed_event`. Money is `int64` USDC cents; bid credits are `int64` whole credits.

## Run

```sh
cp config.example.yaml config.yaml
make generate          # Wire DI + mocks
go test ./...
go run ./cmd/app --config ./config.yaml
```

Config env (koanf `APP_`-prefixed): `APP_DATASOURCE_POSTGRES_DSN` (db `auction_passive`),
`APP_DATASOURCE_NATS_DSN`, `APP_BIDS_BASEURL` (default `http://bids:8080`), `APP_SERVER_HTTP_ADDR`
(default `:8080`), `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT`. Boots end-to-end via
`deploy/docker-compose.yml`.
