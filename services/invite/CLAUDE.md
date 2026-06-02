# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Dauction `invite` service (this service)

> Owns single-use invite codes, redemption, and the invite chain. Governed by the
> repo-root `CLAUDE.md` (binding: §0 golden rules, §1 glossary, §2 topology, §6 HTTP, §7 data).
> Reaches other services only through `proto/` events / their APIs — never their DBs.

**Owned DB `invite` — tables (`migrations/000001_invite.*.sql`):**
- `invite` — `id, code (unique), issuer_account_id, status, created_at, redeemed_at, redeemed_by`.
  `status` CHECK-constrained to the state machine; a redemption-consistency CHECK ties
  `redeemed_at`/`redeemed_by` to `status='REDEEMED'`.
- `invite_edge` — the chain: one row per redemption `(code, inviter_account_id, invitee_account_id, created_at)`.
- `outbox` — transactional outbox `(event_id, idempotency_key (unique), producer, type, version, occurred_at, payload, published_at)`.

**Invite state machine** (enums MONOSPACE_UPPERCASE):
`ISSUED → REDEEMED` (terminal) | `ISSUED → REVOKED` | `ISSUED → FLAGGED`.
Single-use is enforced at the DB level (conditional `UPDATE … WHERE status='ISSUED'`);
any non-ISSUED / missing code on redeem or revoke/flag → `biz.ErrResourceInvalid`
(missing code on revoke/flag → `ErrResourceNotFound`).

**Routes** (handlers self-register Go 1.22 method patterns, mounted under `/apis` by the gateway):
- `POST /apis/invites/redeem` — body `{ code }`; redeemer id from `X-Account-Id` header (gateway-injected). Atomic single-use redemption.
- `POST /apis/invites` — MEMBER/VIP issues a code; per-issuer quota from config (`invite.issueQuota`, default 5, `<=0` = unlimited).
- `GET /apis/admin/invites` — list/filter (`status`, `issuer`, `limit`, `offset`).
- `POST /apis/admin/invites/{code}/revoke` — ISSUED → REVOKED.
- `POST /apis/admin/invites/{code}/flag` — ISSUED → FLAGGED.
- `GET /apis/admin/invites/chain/{accountId}` — invite chain (edges where the account is the inviter).

**Emits:** `invite.redeemed` `{ code, redeemed_by, issued_by }` (proto `dauction.events.v1.InviteRedeemed`),
via the **transactional outbox**: the status flip + chain edge + outbox row commit in one pgx tx
(`internal/repo/invite.go`), and a background `OutboxPublisher` (`internal/repo/outbox_publisher.go`)
relays unpublished rows to NATS JetStream (subject = event `type`). Producer-stable
`idempotency_key = "invite.redeemed:<code>:<redeemer>"` lets consumers dedup.

**Consumes:** nothing.

**Config keys read** (`config.example.yaml`, all `APP_`-overridable): `server.http.addr` (`:8080`),
`invite.issueQuota`, `datasource.postgres.dsn` (→ `pg-invite:5432/invite`), `datasource.nats.dsn`
(→ `nats:4222`), `collector.exporters.otlp.endpoint` (→ `jaeger:4317`).

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
