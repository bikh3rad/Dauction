# Agent — `bids` service

You own **only** `services/bids/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** Bid-credit wallet, packages, purchase, idempotent debit-on-bid.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** `bid_wallet`, `bid_purchase`, `bid_debit`.
**Routes:** `GET /apis/bids/wallet`, `POST /apis/bids/buy` { packageId }; internal `Debit(member,1,idemKey)`.
**Emits:** `bids.purchased`, `bids.debited`.
**Consumes:** (called sync by auction-passive before a bid is recorded).
**Logic:** credit = int64, $1 each. Packages 100→$80, 50→$45, 20→$20 (record the USDC charge and the
credit grant atomically). `Debit` MUST be idempotent on idemKey; out of credits → `ErrResourceInvalid`.

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
