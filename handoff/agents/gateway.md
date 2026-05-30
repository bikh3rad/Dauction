# Agent — `gateway` service

You own **only** `services/gateway/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** Edge: authN, tier/KYC guard, routing, rate-limit, request fan-in. Build LAST.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** no domain data.
**Routes:** mounts all `/apis/*`, proxying to services.
**Logic:** compose middleware via `pkg/middlewares.MultipleMiddleware` — auth, then a tier+KYC guard fed
by identity/kyc read models (public routes: gallery + lot reads; participation routes require
MEMBER/VIP + APPROVED). Keep the template's two recovery middlewares. Add rate-limiting. No business
logic here — only edge concerns.

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
