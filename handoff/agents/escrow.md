# Agent — `escrow` service

You own **only** `services/escrow/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** The funds ledger + state machine. Sole writer of escrow rows.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** `escrow_ledger`.
**Routes:** `POST /apis/escrow/{tradeId}/fund`, `/confirm`, `GET /apis/escrow/{tradeId}`.
**Emits:** `escrow.locked|released|forfeited|refunded`.
**Consumes:** `escrow.lock_requested`, `auction.hammer`, `auction.won`.
**State:** UNLOCKED→DEPOSIT_LOCKED→FULL_LOCKED→HELD→RELEASED; branches: loser→refund, winner-miss→FORFEITED.
**Logic:** Dutch reserve(10%)→full-lock(100%); passive winner funds cleared price + premium into HELD on
`auction.won`. Losers refunded ≤5min of hammer; winner funds within 24h or FORFEIT. Release on buyer
confirm → seller paid 100% cash or 110% Vault Credit.
**Required test:** fuzz a transition sequence and assert
Σ(locked+released+refunded+forfeited+fees+premium+inspector_fee) is constant once locked.

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
