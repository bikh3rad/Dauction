# Dauction — catalog service

The **catalog** bounded context for [Dauction](https://github.com/bikh3rad/Dauction): it owns
**lots, the weekly 32-lot cap, the certification gate (inspector attestation), and the public gallery
reads**. Built from the `mequq/go-template` clean-architecture template (Go module name `application`).

Each service owns its own database and integrates only through events (NATS/JetStream) and each
owner's sync API — never DB→DB. Catalog owns the `catalog` Postgres DB.

## Responsibilities

- Project a vault `object.listed` event into a **DRAFT** `lot`, carrying the chosen auction type
  (`atype`), the timed `duration_days`, the reserve, and the appraised value.
- Record inspector **attestations** (`PASS`/`FAIL`) — the evidence for the certification gate.
- **Certify** a lot (`DRAFT → CERTIFIED`), gated on an existing PASS attestation.
- **Schedule** a lot (`CERTIFIED → SCHEDULED`) into an ISO week, enforcing the **weekly 32-lot cap**.
- Serve the public weekly gallery and lot detail.

### Lot lifecycle & gates

State machine (catalog-owned portion): `DRAFT → CERTIFIED → SCHEDULED`, with `REJECTED` terminal.
Illegal transitions return `ErrResourceInvalid`.

- **Certification gate** — a lot cannot be CERTIFIED (and thus cannot be SCHEDULED) without at least
  one `PASS` attestation. A `FAIL` attestation on a DRAFT lot rejects it.
- **Weekly 32-cap** — at most 32 lots may be SCHEDULED per ISO week. Enforced atomically in
  `repo.ScheduleTx`: the conditional `UPDATE` fires only when the lot is still CERTIFIED **and** the
  same-week SCHEDULED count is below 32. Exceeding it returns `ErrResourceInvalid`.

Money is `int64` USDC cents. Enums are MONOSPACE_UPPERCASE strings (the value string is the wire code).
API payloads are language-neutral; the React client localizes title/description and formats money/dates.

## HTTP surface (mounted under `/apis` by the gateway)

| Method & path | Purpose |
|---|---|
| `GET  /apis/gallery/weekly` | **Public.** This ISO week's SCHEDULED lots + the supply cap (32). `?week=YYYY-Www` optional. |
| `GET  /apis/lots/{id}` | **Public.** Lot detail: atype, params, state, and the attestation summary. |
| `GET  /apis/admin/lots` | Admin lot list, filter by `?state=` and/or `?week=`. |
| `POST /apis/admin/lots/{id}/attest` | Record an attestation `{inspectorId, result, notesRef}`. Emits `attestation.recorded`. |
| `POST /apis/admin/lots/{id}/certify` | DRAFT→CERTIFIED (requires a PASS attestation). Emits `lot.certified`. |
| `POST /apis/admin/lots/{id}/schedule` | CERTIFIED→SCHEDULED `{scheduledAt?}` (weekly 32-cap). Emits `lot.scheduled`. |
| `GET  /healthz/liveness`, `/healthz/readiness` | Lifecycle health checks. |
| `GET  /metrics` | Prometheus exposition. |
| `GET  /swagger/` | Swagger UI. |

## Events

**Emits** (via the transactional outbox, so the state change and the event commit atomically):

| Subject | Payload (`dauction.events.v1`) | When |
|---|---|---|
| `attestation.recorded` | `{attestation_id, lot_id, inspector, pass}` | An inspector attestation is recorded. |
| `lot.certified` | `{lot_id, attestation_id}` | A lot passes the certification gate. |
| `lot.scheduled` | `{lot_id, object_id, mode, duration_days, scheduled_at, reserve_cents, week}` | A lot is admitted to a week's gallery. |

**Consumes** (idempotent via the `consumed_event` inbox, subject-scoped key):

| Subject | Effect |
|---|---|
| `object.listed` (vault) | Create a DRAFT `lot` carrying atype + durationDays + reserve/appraised value. |

A background `OutboxPublisher` polls the `outbox` table and relays unpublished rows to NATS/JetStream
(at-least-once; consumers dedup on `idempotency_key`). The inbound consumer drains the durable
`catalog` consumer over `object.listed` and acks on success / naks for redelivery on error.

## Data

Raw parameterized pgx SQL with an OTel tracer (per the template's repo style). Owned tables: `lot`,
`attestation`, `outbox`, `consumed_event`. Money columns are `BIGINT` (USDC cents). State columns are
CHECK-constrained to the machine above. See `migrations/`.

## Configuration

`config.example.yaml` targets `deploy/docker-compose.yml` (`pg-catalog:5432` db `catalog`, `nats:4222`
stream `DAUCTION` durable `catalog`, `jaeger:4317`). Override with `APP_`-prefixed env vars, e.g.
`APP_DATASOURCE_POSTGRES_DSN`, `APP_DATASOURCE_NATS_DSN`, `APP_SERVER_HTTP_ADDR` (default `:8080`).

```sh
cp config.example.yaml config.yaml
make generate          # regenerate Wire DI graph (after any wire.go edit)
make swagger           # regenerate ./docs
go test ./...          # table-driven biz + handler tests
go run ./cmd/app --config ./config.yaml
```

## Tests

Table-driven `biz` tests (external `biz_test` package, repo mocked with mockery) cover the
certification gate, the weekly 32-cap boundary (32nd OK / 33rd rejected), the state-machine walk and
illegal jumps, and idempotent `object.listed` consumption. Handler tests cover the public gallery /
lot-detail reads and the admin attest/certify/schedule paths (happy + error). `UsecaseLot`'s mock
lives in `internal/mocks/usecase/` to avoid a `biz → mocks → biz` import cycle.
