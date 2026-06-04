# Dauction — Bids Service

The **bid-credit wallet** bounded context: credit packages, purchase, and the idempotent
debit-on-bid that `auction-passive` calls before recording a passive bid. Built from the
`mequq/go-template` clean-architecture template (Go module `application`). See `CLAUDE.md` for the
binding service contract and the root `Dauction/CLAUDE.md` §5 for the bid economy.

## What it owns

- **Bid credits** are `int64` WHOLE credits ($1 each) — a distinct unit from USDC cents. The two are
  never mixed in one field or column.
- **Packages** (seeded): `PKG_100` → $80, `PKG_50` → $45, `PKG_20` → $20.
- **Tables** (`bids` DB): `bid_package`, `bid_wallet`, `bid_purchase`, `bid_debit`, `outbox`.

## HTTP surface (mounted under `/apis` by the gateway)

| Method & path | Purpose |
|---|---|
| `GET  /apis/bids/wallet` | Caller's balance + recent purchases/debits (subject from `X-Account-Id`). |
| `GET  /apis/bids/packages` | Public package catalogue. |
| `POST /apis/bids/buy` | `{packageId, idempotencyKey?}` — atomic purchase; emits `bids.purchased`. |
| `POST /apis/internal/bids/debit` | `{accountId, amount, idempotencyKey, auctionId}` — idempotent debit-on-bid; emits `bids.debited`. |

Insufficient balance on debit → `RESOURCE_INVALID` ("out of credits"). A debit replay with the same
`idempotencyKey` returns the original debit (HTTP 200, same body) and burns nothing.

## Events

- **Emitted** (transactional outbox → NATS/JetStream): `bids.purchased`, `bids.debited`.
- **Consumed:** none — the service is called synchronously by `auction-passive`.

## Concurrency & atomicity

A debit performs, in ONE transaction: a UNIQUE `bid_debit` insert (the idempotency gate) + a
conditional `UPDATE bid_wallet SET balance_credits = balance_credits - $n WHERE account_id = $1 AND
balance_credits >= $n` + the `bids.debited` outbox row (built once the post-debit balance is known).
The conditional UPDATE matching no row means insufficient funds. A purchase commits the credit grant,
the `bid_purchase` row, and the `bids.purchased` outbox row together, gated on the outbox unique
`idempotency_key`.

## Develop

```sh
make generate   # Wire DI + go.mod tidy (run after any wire.go edit)
make swagger     # regenerate ./docs from swag annotations
make check       # lint
go test ./...    # table-driven biz tests (repo mocked) + handler tests
cp config.example.yaml config.yaml && go run ./cmd/app --config ./config.yaml
```

Config env is `APP_`-prefixed (e.g. `APP_DATASOURCE_POSTGRES_DSN`); defaults in
`config.example.yaml` target `deploy/docker-compose.yml` (`pg-bids:5432` db `bids`, `nats:4222`
stream `DAUCTION` durable `bids`, `jaeger:4317`), HTTP `:8080`.
