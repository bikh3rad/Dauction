# Dauction — Escrow service ("the heart")

The funds ledger + escrow state machine for [Dauction](https://github.com/bikh3rad/Dauction). Escrow
is the **sole writer** of escrow ledger rows and enforces the funds-conservation invariant (root
`CLAUDE.md` §4). Go module name: `application`. Built from the `mequq/go-template` clean-architecture
template (Wire DI, koanf config, OTel, slog).

## Responsibilities

- Owns the `escrow` Postgres DB: `escrow_trade` (state head) + the append-only `escrow_ledger`
  (signed USDC-cent rows; per-(trade, participant) balance = `SUM(amount_cents)`), plus `outbox`
  and `consumed_event`.
- Drives the funds state machine
  `UNLOCKED → DEPOSIT_LOCKED → FULL_LOCKED → HELD → RELEASED` with `REFUNDED` / `FORFEITED` /
  `DISPUTED` branches. Illegal transitions → `biz.ErrResourceInvalid`.
- **Dutch path:** reserve (10%) → full-lock (100%) → hammer → HELD → buyer confirms → RELEASED.
- **Passive path:** `auction.won` creates the trade UNLOCKED with a 24h funding deadline; the winner
  funds price + premium straight into HELD.
- Losers are refunded; a winner who misses the funding window forfeits.
- Release: 100% cash to the seller, or a 110% Vault-Credit instruction on the event.
- Dispute court rulings (`REFUND_BUYER` / `RELEASE_SELLER` / `SPLIT`) applied to HELD/DISPUTED trades;
  SPLIT halves the pot, odd cent to the buyer.

## The conservation invariant (§4)

Once funds are locked, the gross **inflows** (`DEPOSIT_LOCK`, `FULL_LOCK`, `HOLD`) are constant, the
gross **disbursements** (`RELEASE`, `REFUND`, `FORFEIT`, `FEE`, `PREMIUM`, `INSPECTOR_FEE`) never
exceed inflows, and they balance exactly at settlement. Enforced in `biz` on every transition and
asserted by a seeded property/fuzz test (`internal/biz/conservation_fuzz_test.go`).

## HTTP surface (mounted under `/apis` by the gateway)

| Method | Path | Purpose |
|---|---|---|
| GET  | `/apis/escrow/{tradeId}` | trade state + per-participant balances + conservation |
| POST | `/apis/escrow/{tradeId}/fund` | winner funds obligation → HELD (or FORFEITED past deadline) |
| POST | `/apis/escrow/{tradeId}/confirm` | buyer confirms delivery → RELEASED |
| POST | `/apis/admin/escrow/{tradeId}/refund` | loser / manual refund → REFUNDED |
| POST | `/apis/admin/escrow/{tradeId}/forfeit` | manual forfeit → FORFEITED |

Locking (reservation / full-lock) is **event-driven** via `escrow.lock_requested`, not HTTP.

## Events

- **Emits** (outbox → NATS): `escrow.locked`, `escrow.released`, `escrow.forfeited`, `escrow.refunded`.
- **Consumes** (inbox-idempotent): `escrow.lock_requested`, `auction.hammer`, `auction.won`,
  `dispute.resolved`.

## Develop

```sh
cp config.example.yaml config.yaml
make generate    # Wire DI + go.mod tidy (after any wire.go edit)
make swagger     # regenerate ./docs from swag annotations
make check       # lint
go test ./...    # unit + property/fuzz + handler tests
go run ./cmd/app --config ./config.yaml
```

Boots end-to-end (Postgres + NATS + Jaeger) via `deploy/docker-compose.yml`. Config env is
`APP_`-prefixed (see `config.example.yaml`): db `escrow` on `pg-escrow:5432`, NATS stream `DAUCTION`
durable `escrow`, OTLP `jaeger:4317`, HTTP `:8080`.
