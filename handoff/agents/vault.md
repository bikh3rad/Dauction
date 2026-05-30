# Agent — `vault` service

You own **only** `services/vault/` (a clone of `github.com/mequq/go-template`, Go module `application`)
plus **read-only** `proto/`. Do not edit sibling services. Read the repo-root `CLAUDE.md` first — it is
binding (topology §2, state machines §3, escrow §4, bid economy §5, routes §6, conventions §0).

**Role.** Private collections, buyback, vault-credit ledger, list-to-auction.

**Rules.** Replicate the template's `placeholder` vertical slice for every resource
(entity → biz interfaces+impl → repo raw pgx `$1` → dto+validate → handler implements `service.Handler`
+ swag → wire). `make generate` after any wire.go edit; never edit `wire_gen.go`. Money = int64 USDC
cents; bid credits = int64. Own your DB only — reach other services via `proto/` events (NATS/JetStream)
or their API, never their tables. Errors via `dto.HandleError` + biz sentinels; state enums
MONOSPACE_UPPERCASE; transitions validated (illegal → `ErrResourceInvalid`).


**Owns:** `vault_object`, `vault_credit_ledger`.
**Routes:** `GET /apis/vault`, `POST /apis/vault/objects`,
`POST /apis/vault/objects/{id}/list` { atype: dutch|vickrey|uniqbid, durationDays?: 2|5|7 },
`POST /apis/vault/objects/{id}/buyback` { mode: cash|credit }.
**Emits:** `object.listed` { objectId, atype, durationDays? }, `credit.changed`.
**Consumes:** `auction.completed` → settle ownership; release events that credit the seller (100% cash
or 110% Vault Credit).
**Logic:** buyback = 50% cash or 85% Vault Credit. `durationDays` is required for timed (vickrey/uniqbid),
forbidden for dutch. Object states IN_VAULT → APPRAISING → IN_AUCTION → SOLD.

**Definition of done.** Vertical slice(s) wired (`make generate`); migrations + isolated DB; events
match `proto/` with outbox + idempotency keys; swag + `make swagger`; table-driven biz tests with the
repo mocked (mockery); `make check` + `go test ./...` green; boots via `deploy/docker-compose.yml`.
Land one PR; in the summary list the tables, routes, state transitions, and events you added.
