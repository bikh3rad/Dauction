# Dauction

**Dauction** is an invitation-tiered, escrow-backed luxury auction house for the GCC market.
Everyone browses the weekly gallery as a `GUEST`; redeeming an invite code elevates to `MEMBER`
(the house may grant `VIP`), and participation requires KYC approval. Members list objects from
their private **vault** and the platform runs three auction engines — **Dutch** (live descending
price), **Vickrey** (sealed second-price) and **UniqBid** (lowest unique price) — with an
append-only **escrow** ledger that enforces funds conservation end to end. Money is `int64` USDC
cents and bid credits are a separate `int64` unit; the API is language-neutral and the React client
owns all copy in **en · fa · ar · tr** (with RTL for fa/ar).

## Service map

Each bounded context is its own Go microservice (a [`mequq/go-template`](https://github.com/mequq/go-template)
clone), behind one API gateway, integrating **only** through the `proto/` event + DTO contract —
never DB → DB. Full topology, state machines, the escrow invariant, and conventions are in
**[`CLAUDE.md`](./CLAUDE.md)** (binding).

| Service | Owns |
|---|---|
| `gateway` | edge routing, authN, tier/KYC guard, rate-limit |
| `identity` | accounts, tiers, sessions, preferred locale |
| `invite` | codes, redemption, invite chain |
| `kyc` | doc refs, GCC OTP, approval queue |
| `vault` | objects, buyback, vault-credit ledger, list-to-auction |
| `catalog` | lots, weekly 32-cap, certification gate, gallery reads |
| `auction-dutch` | live descending engine, reservation, full-lock, hammer |
| `auction-passive` | Vickrey + UniqBid: timed sealed bids, close & resolve |
| `bids` | bid-credit wallet, packages, debit-on-bid |
| `escrow` | the funds ledger + state machine, refunds, release |
| `dispute` | dispute court, rulings |
| `notifier` | realtime fan-out (WS/SSE), countdowns |

## Layout

```
Dauction/
  services/   # one go-template clone per service (built in later phases)
  web/        # React frontend (Vite + TS), built in Phase F
  proto/      # shared event + DTO contracts (.proto) — FROZEN  (see proto/README.md)
  i18n/       # message catalog + locale policy — FROZEN        (see i18n/keys.md)
  deploy/     # docker-compose: pg-per-service + NATS(JetStream) + Jaeger
  CLAUDE.md   # architecture & rules (binding)
```

## Quick start

```sh
make up        # pg-per-service + NATS(JetStream) + Jaeger   (Jaeger UI → http://localhost:16686)
make check     # i18n key-parity + proto lint + go vet across services
make down      # tear down infra
```

`make help` lists every target.

## How to run a service agent

The system is built **one Claude Code session per service**, each scoped to `services/<name>/` plus
read-only `proto/`. Phase 0 (this scaffold — monorepo, frozen `proto/` + `i18n/`, infra) comes first;
then agents are spawned in dependency order. Per-service kickoff prompts and the ordering live in
**[`handoff/agents/`](./handoff/agents/)** (see its `README.md`). Each agent stays in its folder,
integrates only via `proto/` events + the owner service's API, and merges only when its Definition
of Done (CLAUDE.md §10) is green.

## Contracts are frozen

`proto/` and `i18n/` are the only shared surface every agent depends on. Change them via **dedicated
PRs** only, and keep changes additive. The `make check` key-parity gate blocks i18n catalogs that
drift out of sync.
