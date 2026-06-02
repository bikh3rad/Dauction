# CLAUDE.md

> **Service: `kyc`** — identity verification (Emirates-ID / passport doc refs, GCC phone OTP, admin
> approval queue). Owns its DB `kyc`; talks to other contexts only via `proto/` events. The
> repo-root `CLAUDE.md` is binding (golden rules §0, topology §2, state machines §3, routes §6,
> data §7). The template-plumbing guidance below (Wire / koanf / OTel / vertical slice) still applies.

## Service map (kyc)

**Tables (own DB `kyc`):**
- `kyc_submission` — one attempt per row (`account_id, doc_type, doc_ref, phone, state,
  rejection_reason, decided_by, submitted_at, decided_at`); `state` CHECK-constrained.
- `otp_challenge` — OTP per submission (`code_hash` only, `attempts`, `verified`, `expires_at`).
- `outbox` — transactional outbox (`idempotency_key` UNIQUE, `subject`, `payload`, `published_at`).

**State machine:** `STARTED → OTP_VERIFIED → SUBMITTED → APPROVED | REJECTED`. (The repo advances
STARTED straight to SUBMITTED on OTP verify; OTP_VERIFIED is reserved/valid.) REJECTED is terminal →
account may resubmit (new STARTED row). APPROVED or in-flight blocks a new `start`. Illegal
transition → `biz.ErrResourceInvalid`.

**Routes (under `/apis`, caller from `X-Account-Id` header set by the gateway):**
`POST /apis/kyc/start`, `POST /apis/kyc/verify`, `GET /apis/kyc/status`,
`GET /apis/admin/kyc`, `POST /apis/admin/kyc/{id}/approve`, `POST /apis/admin/kyc/{id}/reject`.
Errors are language-neutral codes via `dto.HandleError`.

**Events emitted (NATS JetStream, payloads per `proto/dauction/events/v1`):**
`kyc.approved` (`KycApproved{account_id, submission_id}`),
`kyc.rejected` (`KycRejected{account_id, submission_id, reason}` where reason is an `ErrorCode`
value-name). Wrapped in `EventEnvelope` (producer=`kyc`); written to `outbox` in the same tx as the
decision; a background relay (`biz.OutboxRelay`, registered on the app Controller) publishes them.
**Consumes:** nothing.

**OTP rules:** 6-digit, hash-only storage, default 5-min TTL and 5-attempt cap (config
`kyc.otpTtlSeconds` / `kyc.maxAttempts`; `kyc.production` suppresses the dev-code log/response).

---

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
