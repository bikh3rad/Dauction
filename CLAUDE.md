# CLAUDE.md — Dauction (microservices)

Guidance for Claude Code building **Dauction**: an invitation-tiered, escrow-backed luxury
auction platform for the GCC market. **Each bounded context is its own Go microservice**, each
built from the [`mequq/go-template`](https://github.com/mequq/go-template) clean-architecture
template (Go module name `application`), behind one API gateway, with a **React** frontend.

> **Repo:** `git@github.com:bikh3rad/Dauction.git` (public). Monorepo layout in §8.
> **Authoritative for** domain rules, service boundaries, state machines, the bid economy, and
> conventions. The Go template ships its own CLAUDE.md for Wire/koanf/OTel plumbing — keep it in
> each service; this file governs the system.

---

## 0. Golden rules

1. **One service = one bounded context = one Claude Code agent** (see §9). A service owns its data
   and exposes a contract; no service reaches into another's database. Cross-service calls are
   explicit (sync gRPC/REST) or events (async via NATS/JetStream).
2. Every service is the template's vertical slice, repeated per resource:
   `entity → biz (Usecase+Repository interfaces) → biz impl → repo impl (raw pgx, $1 params) →
   dto (validate tags + mappers) → handler (implements service.Handler, swag annotations) → wire`.
   Dependency direction one-way: `cmd → app → service/handler → biz → repo → datasource → entity`.
3. **Wire is compile-time.** After any `wire.go` edit run `make generate`. Never edit `wire_gen.go`.
4. **Money is `int64` USDC cents.** No floats for money, ever. **Bid credits are `int64` whole credits**
   ($1 each) — a separate unit from USDC; never mix the two in one field.
5. **Escrow service is the only writer of escrow ledger rows** and enforces funds-conservation (§4).
6. **State transitions are explicit + validated**; illegal transition → `biz.ErrResourceInvalid`.
7. **API is language-neutral** (enum codes, integer amounts, ISO-8601 UTC). All `en / fa / ar / tr`
   copy and `LTR/RTL` live in the React client (`fa`/`ar` = RTL; `en`/`tr` = LTR).

---

## 1. Domain glossary

- **Access tiers** — everyone browses the gallery without an account (`GUEST`). Redeeming an **invite
  code** elevates to `MEMBER` (one tier up); the house may grant `VIP`. Participation needs
  `MEMBER`/`VIP` **and** KYC-approved.
- **Invite / KYC** — single-use codes with an invite chain; Emirates-ID/passport + GCC OTP.
- **Vault** — a member's private collection. Owner can **list** an object (choosing the auction type
  and, for timed auctions, the **duration** — 2 / 5 / 7 days), or take **instant buyback** (50% cash
  or 85% Vault Credit).
- **Auction modes** (the platform runs all three):
  - **Dutch** — *live, active*: price descends from a ceiling by `drop_step` every `drop_interval`
    until the first buyer hits *buy* or it reaches the floor. Server-authoritative price.
  - **Vickrey** — *timed, passive*: sealed second-price. One hidden bid per participant; the
    **second-highest** price wins, paid at that price; ties → earliest bid. Sealed until close.
  - **UniqBid** — *timed, passive*: **lowest unique** price wins. A participant may submit many
    distinct prices; the lowest price chosen by exactly one participant takes the lot.
- **Timed window** — Vickrey/UniqBid auctions close at an owner-set deadline (2/5/7 days).
- **Bid economy** — submitting a bid in a **passive** auction spends one **bid credit**. Credits are
  bought in packages (each credit = $1.00): **100 → $80**, **50 → $45**, **20 → $20**. Dutch auctions
  do **not** consume credits (they use the escrow deposit flow instead).
- **Escrow / settlement / release** — Dutch: 10% reservation deposit → 100% lock at open → hammer →
  losers refunded ≤ 5 min → winner funds in 24 h (else forfeit) → buyer confirms delivery → release
  (100% cash or 110% Vault Credit). Passive: winner's obligation is the cleared price + premium,
  funded into escrow on close, then the same delivery→release tail.
- **Roles** — Buyer, Seller, Inspector (attestation seal), House/Admin (invites, KYC, certification,
  dispute court), and the Escrow service itself.

---

## 2. Microservice topology

| Service | Owns | Sync API (excerpt) | Emits events | Consumes |
|---|---|---|---|---|
| `gateway` | edge routing, authN, tier/KYC guard, rate-limit, request fan-in | all `/apis/*` | — | — |
| `identity` | accounts, tiers, sessions | `GET /me`, tier reads | `account.tier_changed` | `invite.redeemed`, `kyc.approved` |
| `invite` | codes, redemption, invite chain, revoke/flag | `POST /invites/redeem`, admin list | `invite.redeemed` | — |
| `kyc` | doc refs, OTP, approval queue | `POST /kyc/start|verify`, admin queue | `kyc.approved`/`kyc.rejected` | — |
| `vault` | objects, buyback, vault-credit ledger, **list-to-auction** | `GET/POST /vault/*` | `object.listed`, `credit.changed` | `auction.completed` |
| `catalog` | lots, weekly 32-cap, certification gate, gallery reads | `GET /gallery/weekly`, `GET /lots/{id}` | `lot.certified`, `lot.scheduled` | `object.listed`, `attestation.recorded` |
| `auction-dutch` | live descending engine, reservation, full-lock, hammer | `POST /auctions/{id}/reserve|lock|buy`, WS price feed | `auction.opened|hammer|completed` | `lot.scheduled`, `escrow.locked` |
| `auction-passive` | Vickrey + UniqBid: timed bids, sealed store, close & resolve | `POST /auctions/{id}/bid`, `GET .../standing` | `bid.placed`, `auction.closed`, `auction.won` | `lot.scheduled`, `bids.debited` |
| `bids` | bid-credit wallet, packages, purchase, debit-on-bid | `GET /bids/wallet`, `POST /bids/buy` | `bids.purchased`, `bids.debited` | `bid.placed` |
| `escrow` | the funds ledger + state machine (§4), refunds, release | `POST /escrow/{id}/fund|confirm`, `GET /escrow/{id}` | `escrow.locked|released|forfeited` | `auction.hammer`, `auction.won` |
| `dispute` | dispute court, rulings | `POST /escrow/{id}/dispute/resolve` | `dispute.resolved` | buyer claim |
| `notifier` | realtime fan-out (WS/SSE), countdowns, toasts | `WS /live/*` | — | most domain events |

**Rules of engagement.** Services talk **use-case → contract**, never DB → DB. `auction-passive`
asks `bids` to debit a credit (sync, idempotent) *before* recording a bid, and the bid write +
`bids.debited` are reconciled via an outbox. `escrow` is the sole funds authority; auctions request
locks/holds through it. Price/clock truth is always server-side — clients render from parameters.

---

## 3. Auction state machines (`MONOSPACE_UPPERCASE` enums)

**Dutch** — `DRAFT → APPRAISING → SCHEDULED → OPEN → HAMMER → SETTLING → COMPLETED` (`CANCELLED` /
`ABORTED` if threshold unmet). Entry to `OPEN` requires `kyc=APPROVED ∧ tier∈{MEMBER,VIP} ∧
reservation=LOCKED ∧ full_lock=LOCKED`. `current_price(now)=max(floor, ceiling − step·⌊(now−open_at)/interval⌋)`.

**Passive (Vickrey / UniqBid)** — `DRAFT → APPRAISING → SCHEDULED → OPEN → CLOSING → RESOLVED →
SETTLING → COMPLETED`. `OPEN` accepts bids until `closes_at` (owner duration). Each accepted bid
**must** carry a confirmed `bids.debited`. Resolution at `CLOSING`:
- **Vickrey** — order sealed bids desc; winner = bidder of the **2nd-highest** distinct price, pays
  that price; tie on a price → earliest `placed_at`.
- **UniqBid** — count multiplicity across all bids; among prices with count==1 pick the **minimum**;
  its bidder wins at that price. No unique price ⇒ `ABORTED` (or relist, house policy).

Determinism: resolution is a pure function of the immutable bid log + `placed_at`; write a
table-driven test that fuzzes bid sets and asserts the winner rule (incl. ties and no-unique).

---

## 4. Escrow ledger + funds conservation (the heart)

Append-only ledger; per-(trade, participant) derived balance; `escrow` is the only writer.

```
                 reserve(10%)        open(100%)        hammer/won
   UNLOCKED ──▶ DEPOSIT_LOCKED ──▶ FULL_LOCKED ──▶ HELD ──▶ RELEASED   (→ seller)
        ▲             │                 │           │
        └─ refund ◀───┴── unfreeze(loser)           └─ FORFEITED (winner missed 24h funding)
```

Dutch uses the full reserve→full-lock path. Passive winners skip the live full-lock: on
`auction.won` the cleared price + buyer's premium is funded into `HELD` within the funding window.
**Invariant (enforce in `biz`, assert in tests):** once locked,
`Σ(locked + released + refunded + forfeited + fees + premium + inspector_fee)` is constant.
Dispute court: `HELD/DISPUTED`, manual release suspended, ruling ∈ `{REFUND_BUYER, RELEASE_SELLER, SPLIT}`.

---

## 5. Bid-credit economy (`bids` service)

- `bid_credit` is `int64` whole credits, $1 each. **Never** stored as USDC cents in the same column.
- Packages (seed): `{100,$80}`, `{50,$45}`, `{20,$20}`. Buying credits is a USDC charge; record both
  the USDC debit and the credit grant atomically.
- `POST /auctions/{id}/bid` flow: `auction-passive` calls `bids.Debit(member, 1, idempotency_key)`
  **before** persisting the bid; on debit failure return `ErrResourceInvalid` ("out of credits").
  Emit `bids.debited`; the bid row + event commit via outbox so a credit is never burned without a
  recorded bid (and vice-versa).
- Wallet balance is read-through; never recompute spend in a handler.

---

## 6. HTTP surface (per service, mounted under `/apis` by the gateway)

Follow the template: each handler implements `service.Handler`, self-registers routes (Go 1.22
method patterns, `r.PathValue`), validates request DTOs (`validate:"…"`), maps errors via
`dto.HandleError` + `biz` sentinels (`ErrResourceNotFound|Exists|Invalid|AccessDenied`), and is
appended to `NewServiceList`. Swag-annotate everything (`make swagger`). Indicative routes:

```
# public
GET  /apis/gallery/weekly            GET /apis/lots/{id}
# access
POST /apis/invites/redeem            POST /apis/kyc/start | /apis/kyc/verify
# dutch (live)
POST /apis/auctions/{id}/reserve     POST /apis/auctions/{id}/lock     POST /apis/auctions/{id}/buy
WS   /apis/live/auctions/{id}
# passive (timed)
POST /apis/auctions/{id}/bid         GET  /apis/auctions/{id}/standing  # vickrey: your sealed bid; uniqbid: your prices + lowest-unique
# bids
GET  /apis/bids/wallet               POST /apis/bids/buy   { packageId }
# escrow / settle
POST /apis/escrow/{tradeId}/fund     POST /apis/escrow/{tradeId}/confirm   GET /apis/escrow/{tradeId}
# vault / seller
GET  /apis/vault   POST /apis/vault/objects   POST /apis/vault/objects/{id}/list   { atype, durationDays? }
POST /apis/vault/objects/{id}/buyback   { mode: cash|credit }
# admin
GET/POST /apis/admin/{invites|kyc|lots|escrow}/...
```

Auth/authorization is gateway middleware composed via `pkg/middlewares.MultipleMiddleware`
(tier + KYC guard). Realtime is a `notifier` WS/SSE handler that only **broadcasts** server-computed
state (`current_price`, `next_drop_at`, `closes_at`, `lowest_unique`, `participants`); the buy/bid
decision is always re-validated server-side. Target < 100 ms.

---

## 7. Data & migrations (per service, isolated DB/schema)

Raw parameterized pgx SQL with an OTel tracer, exactly like `repo/placeholder.go`. Money columns
`BIGINT` (USDC cents); `bid_credit` columns `BIGINT` (credits). Core tables by service: `account`,
`invite`, `invite_edge`; `kyc_submission`; `vault_object`, `vault_credit_ledger`; `lot`, `attestation`;
`auction` (+ `auction_param`), `auction_participant`, `reservation`; `passive_bid` (immutable log,
`placed_at`), `sealed_bid`; `bid_wallet`, `bid_purchase`, `bid_debit`; `escrow_ledger`, `dispute`.
Every state column constrained to the §3/§4 machines. Index "active week, status=OPEN" and
"auction_id, placed_at" for resolution.

---

## 8. Monorepo layout

```
Dauction/
  services/
    gateway/  identity/  invite/  kyc/  vault/  catalog/
    auction-dutch/  auction-passive/  bids/  escrow/  dispute/  notifier/
        └── each = a full go-template clone (cmd/, internal/, migrations/, config.example.yaml, Makefile)
  web/                      # React frontend (Vite + TS), features mirror services
  deploy/                   # docker-compose.yml (pg per service + NATS + jaeger), k8s/ later
  proto/                    # shared gRPC/event contracts (.proto) + generated stubs
  CLAUDE.md                 # this file (root)
  README.md
```

Each service keeps the template's `make generate / swagger / check`, koanf config (`APP_`-prefixed
env), OTel OTLP, and slog. `deploy/docker-compose.yml` brings up every service + its Postgres + NATS
+ Jaeger for local end-to-end.

---

## 9. One agent per microservice (how to run the team)

See `BUILD_PROMPT.md` for the exact kickoff. In short:

- **Contracts first.** A single "platform" agent (or you) authors `proto/` — the event names in §2
  and the DTOs in §6 — and merges it before service agents start. This is the only shared surface.
- **One Claude Code session per service**, each scoped to `services/<name>/` with this root CLAUDE.md
  plus a short service-CLAUDE.md naming that service's tables, states, events emitted/consumed, and
  endpoints. Give each agent **only** its folder + `proto/`; it must not edit siblings.
- **Integration contract is the event/proto schema**, never a shared DB. If an agent needs data it
  doesn't own, it consumes an event or calls the owner's API — add the dependency to §2, don't reach in.
- **Definition of done per service** (§10) must be green before it's merged. The `notifier`, `gateway`,
  and `web` agents come last because they depend on the others' contracts.
- **Frontend agent** builds `web/` from the HTML prototype in this repo as the interaction + visual
  spec (burgundy+gold over warm-black; Source Serif 4 / IBM Plex Sans+Mono / Vazirmatn for fa·ar;
  4 languages `en·fa·ar·tr`; mobile buyer + desktop admin + the cross-role Flow view).

---

## 10. Definition of done (per service)

- [ ] Vertical slice complete for each resource; wired; `make generate` run (no hand-edited `wire_gen.go`).
- [ ] State transitions validated; illegal → `ErrResourceInvalid`. Money `int64` cents; credits `int64`.
- [ ] Owned migrations; isolated DB; no cross-service DB access.
- [ ] Events emitted/consumed match `proto/`; cross-service writes use outbox + idempotency keys.
- [ ] Swag + `make swagger`; errors via `dto.HandleError`.
- [ ] Table-driven `biz` tests with `repo` mocked (mockery). `escrow` asserts funds conservation;
      `auction-passive` asserts Vickrey/UniqBid winner rules (incl. ties / no-unique).
- [ ] `make check` and `go test ./...` green; service boots via `deploy/docker-compose.yml`.
```
