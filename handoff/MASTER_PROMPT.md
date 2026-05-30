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

PHASE 1..N (one service at a time, each its own PR; order by dependency):
  identity, invite, kyc → vault, catalog → bids → auction-dutch → auction-passive → escrow, dispute
  → notifier, gateway. Earlier services may be stubbed via their proto contract.
  For EACH service: build the full vertical slice(s), wire it, `make generate`, swag + `make swagger`,
  write table-driven biz tests with repo mocked. REQUIRED tests:
    • escrow: fuzz a transition sequence, assert funds-conservation invariant holds.
    • auction-passive: assert Vickrey winner (2nd price, ties→earliest) and UniqBid winner
      (lowest unique, no-unique case).
  Validate every state transition (illegal → ErrResourceInvalid). `make check` + `go test ./...` green.

PHASE F (frontend web/ — apply the prototype UI 1:1):
  - Vite + React + TypeScript + TanStack Query against the gateway; thin WS client for the live Dutch
    price feed and passive countdowns. Server truth is never duplicated in client state.
  - Feature folders mirror services: src/features/{gallery,auction-dutch,auction-passive,bids,escrow,
    vault,invite,kyc,membership,admin,flow}; shared primitives in src/ui/ (Seal, StateChip, Money,
    ProductArt, CountdownPill, DocumentChrome, RuleCard, BidWalletStrip).
  - Lift design tokens VERBATIM from theme.css (burgundy+gold over warm-black; keep the `navy` alt
    theme). Fonts: Source Serif 4 (display), IBM Plex Sans (UI), IBM Plex Mono (numerals/state codes),
    Vazirmatn (fa·ar). Keep the octagonal attestation seal + document chrome.
  - i18n = react-i18next with FOUR catalogs en·fa·ar·tr (port every key in i18n.js); dir from language.
    State enum codes are NEVER translated (render mono uppercase).
  - Reproduce these surfaces from the prototype: buyer MOBILE app (gallery, lot, Dutch live auction,
    Vickrey + UniqBid timed screens, bid-package store, escrow flow, vault with list-to-auction,
    membership, account); admin DESKTOP dashboard (invites, KYC, certification, escrow + dispute);
    and the read-only cross-role FLOW view. Match the reserve→full-lock→settle→release escrow strip
    and the bid-credit flows exactly.

═══ DEFINITION OF DONE (every service) ═══
Vertical slice + wired (make generate, no hand-edited wire_gen.go); transitions validated; money int64
cents / credits int64; owned migrations + isolated DB; events match proto/ with outbox + idempotency
keys; swag + make swagger; biz tests with repo mocked (escrow funds-conservation; auction-passive
winner rules); make check + go test ./... green; boots via deploy/docker-compose.yml.

═══ WORKING STYLE ═══
Read the template's placeholder/healthz slices before inventing patterns. One PR per service; in each
PR summary list the state transitions, routes, and events you added. Ask before adding a third-party
dependency (the template already brings pgx, NATS, koanf, OTel, wire, uuid). After Phase 0, stop for
review; then proceed service by service.

START NOW with PHASE 0 only.
────────────────────────────────────────────────────────────────────────
