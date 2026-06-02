# Dauction — `invite` service

Single-use invite codes, redemption, and the invite chain for the Dauction platform.
One bounded context = one Go microservice, built on the `mequq/go-template`
clean-architecture template (Go module `application`). Governed by the repo-root
`CLAUDE.md`; this service owns the `invite` Postgres DB and integrates with the rest of
the platform only through `proto/` events and other services' APIs.

## What this service does

- **Issue** — a `MEMBER`/`VIP` mints a new single-use code (`POST /apis/invites`), subject to a
  per-issuer quota (`invite.issueQuota`, default 5; `<=0` = unlimited).
- **Redeem** — `POST /apis/invites/redeem` atomically consumes an `ISSUED` code for the caller
  (`X-Account-Id`, gateway-injected), records the invite-chain edge, and enqueues an
  `invite.redeemed` event. Single-use is enforced at the DB level; reused/revoked/flagged/missing
  codes → `400`.
- **Admin** — list/filter (`GET /apis/admin/invites`), revoke (`POST …/{code}/revoke`),
  flag (`POST …/{code}/flag`), and read the invite chain (`GET …/chain/{accountId}`).

### State machine

`ISSUED → REDEEMED` (terminal) | `ISSUED → REVOKED` | `ISSUED → FLAGGED`. Illegal transitions →
`biz.ErrResourceInvalid`.

### Tables (DB `invite`)

`invite` (codes + status), `invite_edge` (inviter → invitee chain), `outbox` (transactional
event outbox). See `migrations/000001_invite.up.sql`.

### Events

Emits `invite.redeemed` `{ code, redeemed_by, issued_by }` (proto `dauction.events.v1.InviteRedeemed`,
consumed by `identity` to elevate tier). Delivery uses the **transactional outbox**: the redemption
write and the outbox row commit in one tx; a background publisher relays unpublished rows to NATS
JetStream. Consumes nothing.

### Integration notes (gateway / compose)

- HTTP listen: `:8080`.
- Postgres: `pg-invite:5432/invite` (user/pass `dauction`); NATS: `nats:4222`; OTLP: `jaeger:4317`.
- All config keys are `APP_`-overridable (e.g. `APP_SERVER_HTTP_ADDR`, `APP_INVITE_ISSUEQUOTA`).
- Routes are mounted under `/apis` by the gateway; the gateway supplies the trusted `X-Account-Id`
  header after authN + tier/KYC guard.

---

## Template reference (plumbing)

The Go module is named `application`.

## What's in the box

- **HTTP server** — stdlib `net/http.ServeMux` with `otelhttp` + panic-recovery middleware (`app/httpserver.go`).
- **Compile-time DI** — Google Wire; composition root in `cmd/app/wire.go`.
- **Configuration** — koanf, YAML file plus `APP_`-prefixed env overlay (`app/config.go`).
- **Observability** — OpenTelemetry traces, metrics, and logs over OTLP gRPC; slog bridged to OTel logs (`app/otel.go`, `app/logger.go`).
- **Datasources** — PostgreSQL (pgx + otelsql), an in-memory ramsql DB for testing, and a NATS/JetStream client.
- **Lifecycle** — components self-register `Start`, `Shutdown`, and `Healthz` hooks on a shared `app.Controller` (`app/controller.go`).
- **Mocks** — mockery v2 generates from `internal/biz` into `internal/mocks` (`.mockery.yaml`).
- **API docs** — swag generates Swagger; UI mounted at `/swagger/`.

## Quickstart

```sh
cp config.example.yaml config.yaml
make devtools                      # one-time: golangci-lint, gofumpt, mockgen, swag, gci
make generate                      # tidy go.mod, install wire, run go generate
go run ./cmd/app --config ./config.yaml
```

The server listens on `:8080` by default. Override any config key via env, e.g. `APP_SERVER_HTTP_ADDR=:9090`.

## Endpoints exposed by default

| Path | Notes |
|---|---|
| `GET /healthz/liveness` | Liveness check (fans out across registered checks) |
| `GET /healthz/readiness` | Readiness check |
| `GET /healthz/panic` | Triggers a panic for testing the recovery middleware |
| `GET /healthz/sleep/{time}` | Sleeps for a `time.Duration` (e.g. `10s`) |
| `POST /apis/invites/redeem` | Redeem a single-use invite code |
| `POST /apis/invites` | Issue a new invite code (MEMBER/VIP, quota-limited) |
| `GET /apis/admin/invites` | List/filter invites (admin) |
| `POST /apis/admin/invites/{code}/revoke` | Revoke an ISSUED code (admin) |
| `POST /apis/admin/invites/{code}/flag` | Flag an ISSUED code (admin) |
| `GET /apis/admin/invites/chain/{accountId}` | Invite chain for an account (admin) |
| `GET /metrics` | Prometheus exposition |
| `GET /swagger/` | Swagger UI (spec at `/docs/swagger/swagger.json`) |

## Common commands

```sh
make generate         # regenerate Wire DI graph + go.mod tidy
make swagger          # regenerate ./docs/ from swag annotations
make check            # golangci-lint
make unit_tests       # tests under internal/service/handler and internal/biz
make coverage_tests   # same, with coverage profile to coverage.out
make all_tests        # tests + benchmarks + coverage (JSON output)
make build            # docker build -t buildf .
```

Run a single test:

```sh
go test ./internal/service/handler/... -run TestName -v
```

## Architecture

```
cmd/app                 main + wire composition root
app/                    runtime infra (Application, HTTPServer, Controller, KConfig, Logger, OTLP)
internal/service        HTTP wiring (mux, /metrics, /swagger)
internal/service/handler HTTP handlers — implement service.Handler
internal/service/dto    request/response shapes + error mapping
internal/biz            use cases (interfaces consumed by handlers)
internal/repo           repository implementations (bound to biz Repository interfaces)
internal/datasource     DB / queue clients
internal/entity         domain types
internal/mocks          mockery-generated mocks for internal/biz
pkg/middlewares         per-route HTTP middlewares
infra/                  docker-compose stacks (monitoring, postgres, redis) + k8s manifests
migrations/             SQL migrations
```

Dependency direction: `cmd → app → service/handler → biz → repo → datasource → entity`.

To add a handler: implement `service.Handler` (`RegisterHandler(ctx) error` registers routes on the injected `*http.ServeMux`), expose a `New…` provider, and append it to `NewServiceList` in `internal/service/handler/wire.go`. Then run `make generate`.

## Configuration

`config.yaml` is the source of truth at runtime; copy from `config.example.yaml`. Env vars prefixed `APP_` are merged on top, with `_` mapping to `.` (e.g. `APP_SERVER_HTTP_ADDR` → `server.http.addr`). The `--config` flag selects the file (default `./config.yaml`).

## Local infrastructure

`docker-compose.yml` aggregates includes from `infra/compose/` for monitoring, Postgres, and Redis:

```sh
docker compose up -d
```

The `Tiltfile` references a Helm chart at `./charts/...` that is not included in this repo — add your own chart to use Tilt.
