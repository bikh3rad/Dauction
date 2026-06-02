# Dauction — vault service

The **vault** bounded context for [Dauction](https://github.com/bikh3rad/Dauction): it owns
**members' private collections, instant buyback, the Vault-Credit ledger, and list-to-auction**.
Built from the `mequq/go-template` clean-architecture template (Go module name `application`).

Each service owns its own database and integrates only through events (NATS/JetStream) and each
owner's sync API — never DB→DB. Vault owns the `vault` Postgres DB.

## Responsibilities

- Hold each member's `vault_object` collection (title, description, appraised value, lifecycle state).
- **List to auction**: move an object `IN_VAULT → APPRAISING`, choosing the auction type and (for timed
  auctions) the close window, and emit `object.listed`.
- **Instant buyback**: pay the owner 50% of the appraised value in USDC cash, or credit 85% as Vault
  Credit, moving the object to `BOUGHT_BACK`.
- Maintain the append-only `vault_credit_ledger` (balance = `SUM(delta_cents)`); emit `credit.changed`.
- Settle ownership on `auction.completed` (`IN_AUCTION → SOLD`), crediting the seller when the release
  is paid as Vault Credit.

## Domain rules

- **State machine** (enforced in `biz`, illegal → `RESOURCE_INVALID`): `IN_VAULT → APPRAISING →
  IN_AUCTION → SOLD`, with terminal `IN_VAULT → BOUGHT_BACK`. Repo transitions are atomic conditional
  UPDATEs (`WHERE state = from`).
- **List-to-auction matrix**: `durationDays` (2/5/7) is **required** for `VICKREY`/`UNIQBID` and
  **forbidden** for `DUTCH`.
- **Buyback math**: `CASH = value*50/100`, `CREDIT = value*85/100`, integer math with truncation toward
  zero. All amounts are `int64` USDC cents — never floats. Vault Credit is USDC-denominated cents, a
  separate unit from bid credits.
- **Ownership**: acting on another member's object → `ACCESS_DENIED`.

## Tables (owned, `migrations/`)

| Table | Purpose |
|---|---|
| `vault_object` | one row per collected item; lifecycle state, appraised value (USDC cents) |
| `vault_credit_ledger` | append-only Vault-Credit ledger; signed `delta_cents`; balance = SUM |
| `outbox` | transactional outbox relayed to NATS/JetStream |
| `consumed_event` | inbox / idempotency ledger for consumed events |

## HTTP routes (mounted under `/apis` by the gateway)

| Method & path | Purpose |
|---|---|
| `GET /apis/vault` | caller's objects + Vault-Credit balance |
| `POST /apis/vault/objects` | add an object (`title`, `description`, `appraisedValueCents`) |
| `POST /apis/vault/objects/{id}/list` | list to auction `{atype, durationDays?}`; emits `object.listed` |
| `POST /apis/vault/objects/{id}/buyback` | instant buyback `{mode: CASH|CREDIT}` |

The authenticated subject is read from the gateway-injected `X-Account-Id` header.

## Events

**Emitted** (transactional outbox → NATS/JetStream):
- `object.listed` — `{object_id, owner_id, mode, duration, floor}`
- `credit.changed` — `{account_id, delta, balance, reason}`

**Consumed** (idempotent via the `consumed_event` inbox):
- `auction.completed` — settle `IN_AUCTION → SOLD`; on a Vault-Credit release, append an
  `AUCTION_RELEASE` ledger row and emit `credit.changed`.

## Configuration

koanf config (`APP_`-prefixed env overrides). Defaults in `config.example.yaml` target
`deploy/docker-compose.yml`:

- `APP_DATASOURCE_POSTGRES_DSN` — owned Postgres (`pg-vault:5432`, db `vault`)
- `APP_DATASOURCE_NATS_DSN` — shared event bus (`nats:4222`, stream `DAUCTION`, durable `vault`)
- `APP_SERVER_HTTP_ADDR` — HTTP listen address (default `:8080`)
- `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT` — OTLP collector (`jaeger:4317`)

## Build & test

```sh
make generate     # regenerate Wire DI graph (after any wire.go edit)
make swagger      # regenerate ./docs
go test ./...     # table-driven biz tests + handler tests
make check        # lint
```

See `CLAUDE.md` for the full service contract and the root `CLAUDE.md` for platform-wide rules.
