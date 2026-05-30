# Dauction — Build Prompt (paste into Claude Code)

Kickoff for building **Dauction** as a **Go microservices** backend + **React** frontend, pushed to
`git@github.com:bikh3rad/Dauction.git`. Each microservice is built from
[`mequq/go-template`](https://github.com/mequq/go-template) and **owned by its own Claude Code agent**.
Read the root `CLAUDE.md` first — it defines services, state machines (Dutch / Vickrey / UniqBid),
the escrow funds-conservation invariant, the bid-credit economy, and conventions. It is binding.

---

## What you are building

An invitation-tiered, escrow-backed luxury auction platform (GCC) with **three auction modes**:
**Dutch** (live descending), **Vickrey** (sealed second-price, timed) and **UniqBid** (lowest-unique,
timed). Passive bids cost **bid credits** ($1 each; packages 100/$80, 50/$45, 20/$20). Everyone can
browse; an invite elevates `GUEST→MEMBER`; participation needs KYC. Four languages, server-neutral API.

## Hard constraints

1. Go module stays **`application`** in every service. Replicate the template's `placeholder` slice
   as the pattern (entity → biz interfaces+impl → repo raw pgx → dto+validate → handler → wire);
   `make generate` after any `wire.go` change; **never** edit `wire_gen.go`.
2. **Money = `int64` USDC cents. Bid credits = `int64` credits.** Never floats; never the same column.
3. **One service owns its DB.** No cross-service DB access — call the owner's API or consume its event
   (NATS/JetStream). `escrow` is the only writer of escrow rows and enforces funds conservation.
4. Server-authoritative price/clock and bid resolution. Errors via `dto.HandleError` + `biz` sentinels.
   State enums `MONOSPACE_UPPERCASE`. API language-neutral; all `en·fa·ar·tr` + RTL is client-side.

## Phase 0 — repo + contracts (do this first, alone)

1. `git init`, set remote `git@github.com:bikh3rad/Dauction.git`, create the monorepo layout from
   root `CLAUDE.md` §8 (`services/`, `web/`, `deploy/`, `proto/`). Add root `CLAUDE.md` + `README.md`.
2. Author `proto/` = the **event names** (CLAUDE.md §2) and the **DTOs** (§6). This is the only shared
   surface; freeze it before service agents start.
3. `deploy/docker-compose.yml`: one Postgres per service + NATS (JetStream) + Jaeger.
4. Commit and push `main`. Open a branch protection note so each service lands via its own PR.
   **GitHub push:** `git remote add origin git@github.com:bikh3rad/Dauction.git` then
   `git push -u origin main`. If the repo is empty, this creates it; if SSH isn't set up, use the
   `gh` CLI (`gh repo create bikh3rad/Dauction --public --source=. --remote=origin --push`).

## Phase 1..N — one agent per service (parallelizable after Phase 0)

Spawn a **separate Claude Code session per service**, each scoped to `services/<name>/` + read-only
`proto/`. Give each the root `CLAUDE.md` and a 10-line service note (its tables, states, events
in/out, endpoints). Recommended order (contracts let later ones stub dependencies):

1. `identity`, `invite`, `kyc` — accounts/tiers, single-use codes + chain, OTP + doc approval, tier guard.
2. `vault`, `catalog` — objects + buyback + **list-to-auction** (`atype` + `durationDays`); lots with
   the weekly **32-cap**, certification gate; public gallery reads.
3. `bids` — bid-credit wallet + packages + purchase + **idempotent debit-on-bid**.
4. `auction-dutch` — `DRAFT…COMPLETED`, deterministic `current_price`, reserve(10%)→lock(100%)→buy→hammer; WS feed.
5. `auction-passive` — Vickrey + UniqBid: timed bids (each requires a confirmed `bids.debited`),
   sealed store, close & resolve. **Write the winner-rule fuzz tests** (2nd-price ties; lowest-unique; no-unique).
6. `escrow`, `dispute` — the ledger + machine, refunds ≤5min, 24h funding/forfeit, confirm→release
   (100% cash | 110% credit), dispute rulings. **Write the funds-conservation fuzz test.**
7. `notifier`, `gateway` — realtime broadcast of server-computed state; edge auth + tier/KYC guard
   + routing + rate-limit. (Last: they depend on everyone's contracts.)

Each service agent: stay in your folder, never edit siblings, integrate only through `proto/` events
and the owner's API. Run `make generate` and show the `wire_gen.go` diff (don't edit it). Land via one PR.

## Phase F — frontend (`web/`), apply the prototype UI

The HTML prototype in this repo (`Dauction.html` + `*.jsx`, `theme.css`, `i18n.js`, `data.js`) is the
**interaction + visual spec**. Port it 1:1 to a real app:

- Vite + React + TS + TanStack Query against the gateway; a thin WS client for live Dutch price and
  passive countdowns. Server truth is never duplicated in client state.
- Feature folders mirror services: `src/features/{gallery,auction-dutch,auction-passive,bids,escrow,
  vault,invite,kyc,membership,admin,flow}`; shared primitives in `src/ui/` (Seal, StateChip, Money,
  ProductArt, CountdownPill, DocumentChrome, RuleCard, BidWalletStrip).
- **Lift the design tokens verbatim** from `theme.css` (burgundy+gold over warm-black; the `navy`
  alt theme). Fonts: Source Serif 4 (display), IBM Plex Sans (UI), IBM Plex Mono (numerals/state),
  Vazirmatn (fa·ar). Keep the octagonal attestation seal and document chrome.
- **i18n** = `react-i18next` with **four** catalogs `en·fa·ar·tr`; `dir` from language (`fa`/`ar` RTL,
  `en`/`tr` LTR). Port every key in `i18n.js`. State enum codes are never translated (mono uppercase).
- Surfaces to reproduce: buyer **mobile** app (gallery, lot, **Dutch live auction**, **Vickrey** +
  **UniqBid** timed screens, **bid store**, escrow flow, vault with **list-to-auction**, membership,
  account), admin **desktop** dashboard, and the read-only cross-role **Flow** view. Match the exact
  reserve→full-lock→settle→release escrow strip and the bid-credit flows from the prototype.

## Definition of done (every service) — CLAUDE.md §10

Vertical slice + wired (`make generate`); transitions validated (illegal → `ErrResourceInvalid`);
money `int64` cents / credits `int64`; owned migrations, isolated DB; events match `proto/` with
outbox + idempotency; swag + `make swagger`; table-driven `biz` tests with `repo` mocked — `escrow`
asserts funds conservation, `auction-passive` asserts Vickrey/UniqBid winners; `make check` and
`go test ./...` green; boots via `deploy/docker-compose.yml`.

## Working style

- Prefer reading the template's `placeholder`/`healthz` slices over inventing patterns.
- Keep each PR to one service. Summarize the state transitions, routes, and events you added.
- Ask before adding a third-party dep; the template already brings pgx, NATS, koanf, OTel, wire, uuid.

Start with **Phase 0** only: scaffold the monorepo, author `proto/`, write `deploy/docker-compose.yml`,
push `main` to `git@github.com:bikh3rad/Dauction.git`, then stop for review before spawning service agents.
