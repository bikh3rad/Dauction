# Dauction — auction-dutch service

The **auction-dutch** bounded context for [Dauction](https://github.com/bikh3rad/Dauction): the **live
descending-price (Dutch) auction engine** — reservation deposits, full locks, the server-authoritative
price, and the hammer. Built from the `mequq/go-template` clean-architecture template (Go module name
`application`).

Each service owns its own database and integrates only through events (NATS/JetStream) and each
owner's sync API — never DB→DB. Auction-dutch owns the `auction_dutch` Postgres DB.

## Responsibilities

- Materialize a **SCHEDULED** `auction` from a catalog `lot.scheduled` event (DUTCH lots only).
- Take participant **reservation deposits** (10%) and **full locks** (100%) by asking the escrow
  service to lock funds (`escrow.lock_requested`), then mirroring the confirmed lock on `escrow.locked`.
- Compute the **server-authoritative descending price**
  `current_price(now) = max(floor, ceiling − drop_step·⌊(now−open_at)/interval⌋)`.
- Run the **hammer**: the first eligible `buy` on an OPEN auction wins at the server price
  (`OPEN → HAMMER`); later buys are rejected.
- Drive the admin lifecycle: `SCHEDULED → OPEN` (needs a fully-locked participant), `SETTLING →
  COMPLETED`, and `ABORTED`.

### Routes

- `GET  /apis/auctions/{id}` — public state + live price + next drop.
- `POST /apis/auctions/{id}/reserve` · `/lock` · `/buy` — participant actions.
- `POST /apis/admin/auctions/{id}/open` · `/complete` · `/abort` — house lifecycle.

Caller identity + eligibility come from the gateway headers `X-Account-Id`, `X-Account-Tier`,
`X-Kyc-Approved`. See `CLAUDE.md` for tables, events, the state machine, and the one proto deviation
(`escrow.lock_requested`).

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
| `GET /apis/mocks/placeholders` | Example CRUD resource (list/get/create/update/delete) |
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
