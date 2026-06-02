# Dauction — identity service

The **identity** bounded context for [Dauction](https://github.com/bikh3rad/Dauction): it owns
**accounts, access tiers (GUEST/MEMBER/VIP), and the participation read model** the gateway uses to
authorize tier/KYC-gated routes. Built from the `mequq/go-template` clean-architecture template
(Go module name `application`).

Each service owns its own database and integrates only through events (NATS/JetStream) and each
owner's sync API — never DB→DB. Identity owns the `identity` Postgres DB.

## Responsibilities

- Maintain one `account` record per platform user: its **tier** and a **mirrored KYC status**.
- Serve the gateway's authorization guard a minimal tier + KYC + eligibility read model.
- Elevate tiers in response to platform events, and let the house grant VIP.
- Emit `account.tier_changed` whenever a tier changes (transactional outbox → NATS/JetStream).

### Access tiers & eligibility

Tier only ever **rises**: `GUEST → MEMBER → VIP`. A no-op or downward change is rejected with
`ErrResourceInvalid`. An account is **participation-eligible** when `tier ∈ {MEMBER, VIP}` **and**
`kyc_status = APPROVED`. Tier and KYC enums are MONOSPACE_UPPERCASE strings (the value string is the
wire code). There is no money in this service.

## HTTP surface (mounted under `/apis` by the gateway)

| Method & path | Purpose |
|---|---|
| `GET  /apis/me` | Current account (tier, KYC, `eligible`). Subject from the gateway-injected `X-Account-Id` header. |
| `GET  /apis/internal/accounts/{id}/access` | Minimal tier+KYC+eligible read model for the gateway guard (internal). |
| `POST /apis/admin/accounts/{id}/vip` | House/admin grants VIP. Emits `account.tier_changed`. |
| `GET  /healthz/liveness`, `/healthz/readiness` | Lifecycle health checks. |
| `GET  /metrics` | Prometheus exposition. |
| `GET  /swagger/` | Swagger UI. |

## Events

**Emits** (via the outbox, so the state change and the event commit atomically):

| Subject | Payload (`dauction.events.v1`) | When |
|---|---|---|
| `account.tier_changed` | `AccountTierChanged{account_id, from, to}` | any tier elevation (VIP grant or invite elevation) |

**Consumes** (idempotent via the `consumed_event` inbox, keyed by a subject-scoped idempotency key):

| Subject | Effect |
|---|---|
| `invite.redeemed` | elevate the redeemer GUEST→MEMBER (no-op if already MEMBER/VIP) |
| `kyc.approved` | set the account's `kyc_status = APPROVED` (participation-eligible) |

The **outbox publisher** (`internal/biz/publisher.go`) polls unpublished `outbox` rows and relays
them to NATS, marking each published only after the broker accepts it (at-least-once; consumers
dedup on `idempotency_key`). The **event consumer** (`internal/biz/consumer.go`) subscribes to the
shared JetStream stream and applies inbound envelopes through the account use case.

## Data (owned `identity` DB, `migrations/`)

| Table | Notes |
|---|---|
| `account` | `id` UUID PK, `tier` (CHECK `GUEST\|MEMBER\|VIP`), `kyc_status` (CHECK `PENDING\|APPROVED\|REJECTED`), timestamps. Index `(tier, kyc_status)`. |
| `outbox` | transactional outbox: `id`, `subject`, `idempotency_key` UNIQUE, `payload` JSONB, `created_at`, `published_at`. Partial index on unpublished. |
| `consumed_event` | inbox/idempotency ledger: `idempotency_key` PK, `consumed_at`. |

## Quickstart

```sh
cp config.example.yaml config.yaml
make generate                      # regenerate Wire DI graph + go.mod tidy
go run ./cmd/app --config ./config.yaml
```

Defaults in `config.example.yaml` target `deploy/docker-compose.yml`: Postgres `pg-identity:5432`
(db/user/pass `identity`/`dauction`/`dauction`), NATS `nats:4222`, OTLP `jaeger:4317`, HTTP `:8080`.
Override any key via an `APP_`-prefixed env var (`_` → `.`), e.g. `APP_SERVER_HTTP_ADDR=:9090`,
`APP_DATASOURCE_POSTGRES_DSN=...`, `APP_DATASOURCE_NATS_DSN=...`.

## Common commands

```sh
make generate     # regenerate Wire DI graph + go.mod tidy (run after any wire.go edit)
make swagger      # regenerate ./docs/ from swag annotations
make check        # golangci-lint
go test ./...     # run the test suite
make build        # docker build
```

Never hand-edit `cmd/app/wire_gen.go`; edit the relevant `wire.go` provider set and run `make generate`.

## Architecture

```
cmd/app                  main + wire composition root
app/                     runtime infra (Application, HTTPServer, Controller, KConfig, Logger, OTLP)
internal/service         HTTP wiring (mux, /metrics, /swagger)
internal/service/handler HTTP handlers — implement service.Handler (account, healthz)
internal/service/dto     request/response shapes + error mapping (dto.HandleError + biz sentinels)
internal/biz             use cases: account, event consumer, outbox publisher
internal/repo            repository impls (raw parameterized pgx SQL with an OTel tracer)
internal/datasource      Postgres (pgx + otelsql) and NATS/JetStream clients
internal/eventbus        wires the NATS datasource to the biz publisher/consumer + lifecycle hooks
internal/entity          domain types (Account, Tier, KycState, OutboxEvent)
internal/mocks           mockery-generated mocks for internal/biz
migrations/              SQL migrations (account, outbox, consumed_event)
```

Dependency direction: `cmd → app → service/handler → biz → repo → datasource → entity`.

> The template's own architecture/plumbing notes (Wire, koanf, OTel, lifecycle) live in `CLAUDE.md`.
