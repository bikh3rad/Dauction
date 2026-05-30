────────────────────────────────────────────────────────────────────────
DAUCTION — MASTER BUILD PROMPT  (paste this whole block into Claude Code)
────────────────────────────────────────────────────────────────────────

You are building **Dauction**, an invitation-tiered, escrow-backed luxury auction platform for the
GCC market, end to end: a **Go microservices** backend + a **React** frontend. This folder already
contains an interactive **HTML prototype** (`Dauction.html` + `*.jsx`, `theme.css`, `i18n.js`,
`data.js`) and a `handoff/` folder with `CLAUDE.md`, `BUILD_PROMPT.md`, `AGENTS_AND_GITHUB.md`.

GROUND TRUTH (read these first, in order):
1. `handoff/CLAUDE.md`  — architecture: 12-service topology, auction state machines (Dutch / Vickrey /
   UniqBid), escrow funds-conservation invariant, bid-credit economy, conventions. BINDING.
2. The HTML prototype — the exact **UI + interaction + copy spec**. Port it 1:1.
3. `handoff/BUILD_PROMPT.md` and `handoff/AGENTS_AND_GITHUB.md` — phasing + GitHub steps.

Move `handoff/CLAUDE.md` to the repo root as `CLAUDE.md` before you start.

═══ NON-NEGOTIABLE CONSTRAINTS ═══
• Each microservice is its own clone of the template `github.com/mequq/go-template` (Go module stays
  `application`). Replicate its `placeholder` vertical slice for every resource:
  entity → biz (Usecase+Repository interfaces) → biz impl → repo (raw pgx, $1 params) →
  dto (validate tags + mappers) → handler (implements service.Handler, swag) → wire. One-way deps:
  cmd → app → service/handler → biz → repo → datasource → entity.
• Wire is compile-time: run `make generate` after any wire.go edit; NEVER edit wire_gen.go.
• Money = int64 USDC cents. Bid credits = int64 credits ($1 each). Never floats; never one column.
• One service owns its DB. No cross-service DB access — call the owner's API or consume its NATS
  event. `escrow` is the ONLY writer of escrow rows and enforces funds conservation.
• Server-authoritative price, clock, and bid resolution. Errors via dto.HandleError + biz sentinels
  (ErrResourceNotFound|Exists|Invalid|AccessDenied). State enums MONOSPACE_UPPERCASE.
• API is language-neutral (enum codes, integer amounts, ISO-8601 UTC). All en·fa·ar·tr copy + RTL
  live in the React client (fa/ar = RTL, en/tr = LTR).

═══ DOMAIN (summary — full detail in CLAUDE.md) ═══
• Access: everyone browses; redeeming an invite code elevates GUEST→MEMBER; house may grant VIP.
  Participation needs MEMBER/VIP + KYC-approved (Emirates-ID/passport + GCC OTP).
• Three auction modes:
   – DUTCH (live): price descends from ceiling by drop_step every interval until first BUY or floor.
     Entry = reserve 10% deposit, then lock 100% at open; winner stops the clock at the hammer price.
   – VICKREY (timed, sealed second-price): one hidden bid; 2nd-highest distinct price wins, paid at
     that price; ties → earliest placed_at.
   – UNIQBID (timed, lowest-unique): many distinct prices allowed; the lowest price chosen by exactly
     one bidder wins at that price; no unique → ABORTED/relist.
• Timed auctions close at an OWNER-set duration: 2 / 5 / 7 days (set when listing from the vault).
• Bid economy: each passive bid spends ONE bid credit ($1). Packages: 100→$80, 50→$45, 20→$20.
  Dutch uses escrow deposits, NOT credits.
• Escrow: reserve(10%) → full-lock(100%) → HELD → winner funds hammer+premium in 24h (else FORFEIT)
  → losers refunded ≤5min → buyer confirms delivery → RELEASE (100% cash | 110% Vault Credit).
  Dispute court ruling ∈ {REFUND_BUYER, RELEASE_SELLER, SPLIT}.
• Vault: list-to-auction (choose mode + duration), or instant buyback (50% cash | 85% Vault Credit).
• 32-lot/week supply cap. Certification gate needs an inspector PASS attestation.

═══ SERVICES (monorepo services/<name>/, each a go-template clone) ═══
gateway · identity · invite · kyc · vault · catalog · auction-dutch · auction-passive · bids ·
escrow · dispute · notifier. Integrate ONLY through proto/ events (NATS/JetStream) + each owner's API.
(See CLAUDE.md §2 for each service's tables, sync API, and events emitted/consumed.)

═══ BUILD PLAN ═══
PHASE 0 (do alone, then PAUSE for my review):
  - Create the monorepo: services/ web/ deploy/ proto/ ; put CLAUDE.md + README.md at root.
  - Author proto/ = the event names (CLAUDE.md §2) + DTOs (§6). Freeze it — it's the only shared surface.
  - deploy/docker-compose.yml: one Postgres per service + NATS(JetStream) + Jaeger.
  - Add .gitignore (Go + Node). Commit. Push main to https://github.com/bikh3rad/Dauction
    (git remote add origin https://github.com/bikh3rad/Dauction.git ; git push -u origin main —
     or `gh repo create bikh3rad/Dauction --public --source=. --remote=origin --push`).
  - STOP and show me the layout + proto contracts before building services.


START NOW with PHASE 0 only.
────────────────────────────────────────────────────────────────────────
