# Agent — `auction-passive` service

You own **only** `services/auction-passive/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** Vickrey + UniqBid timed auctions: bids, sealed store, close & resolve.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** `passive_bid` (immutable log with placed_at), `sealed_bid`.
**Routes:** `POST /apis/auctions/{id}/bid` { price }, `GET /apis/auctions/{id}/standing`.
**Emits:** `bid.placed`, `auction.closed`, `auction.won`.
**Consumes:** `lot.scheduled`, `bids.debited`.
**State:** DRAFT→APPRAISING→SCHEDULED→OPEN→CLOSING→RESOLVED→SETTLING→COMPLETED; OPEN accepts bids until
owner-set closes_at (2/5/7 days).
**Logic:** every accepted bid MUST carry a confirmed `bids.Debit` (call bids sync BEFORE persisting;
commit bid row + `bid.placed` via outbox so a credit is never burned without a recorded bid).
Resolution (pure fn of the immutable log + placed_at):
 • VICKREY: winner = bidder of the 2nd-highest DISTINCT price, pays that price; tie on a price → earliest.
 • UNIQBID: among prices with count==1, the minimum wins at that price; no unique → ABORTED/relist.
**Required tests:** fuzz Vickrey (ties→earliest, single-bidder edge) and UniqBid (lowest-unique, no-unique).

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
