# Agent — `auction-dutch` service

You own **only** `services/auction-dutch/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** Live descending-price engine + reservation + full-lock + hammer.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** `auction` (+ `auction_param`), `auction_participant`, `reservation`.
**Routes:** `POST /apis/auctions/{id}/reserve`, `/lock`, `/buy`; WS price feed (delegated to notifier).
**Emits:** `auction.opened|hammer|completed`, `escrow.lock_requested`.
**Consumes:** `lot.scheduled`, `escrow.locked`.
**State:** DRAFT→APPRAISING→SCHEDULED→OPEN→HAMMER→SETTLING→COMPLETED (CANCELLED/ABORTED if threshold unmet).
**Logic:** `current_price(now)=max(floor, ceiling − step·⌊(now−open_at)/interval⌋)` — server-authoritative;
re-validate price on /buy. Entry to OPEN requires kyc=APPROVED ∧ tier∈{MEMBER,VIP} ∧ reservation=LOCKED
∧ full_lock=LOCKED. First valid /buy → HAMMER; reject later buys.

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
