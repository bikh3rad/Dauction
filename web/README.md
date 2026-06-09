# Dauction — Web (mobile-native React frontend)

The buyer-facing client for **Dauction**, the invitation-tiered, escrow-backed
luxury auction platform. Built **mobile-first / app-like**, wired to the Go
microservices behind the gateway, with a schema-accurate **offline mock fallback**
so the app stays fully usable when the backend is down.

> Design fidelity: this is a faithful production port of the repo's HTML/JSX
> prototype (`../Dauction.html` + `*.jsx`) — burgundy + gold over warm-black,
> Source Serif 4 / IBM Plex Sans+Mono / Vazirmatn, the octagonal certification
> seal, mono state-chips, and the editorial product line-art.

## Stack

- **Vite + React 18 + TypeScript**
- **React Router v6** — deep-linkable screens, app-like stack transitions
- **TanStack Query** — fetching, caching, loading/error states, mutations
- **axios** — centralized API client (gateway dev-auth: `Bearer <accountId>`)
- **framer-motion** — native-feel page slides + bottom sheets
- **i18n** — 4 languages `en · fa · ar · tr`, `LTR/RTL` (fa·ar RTL), reusing the
  prototype's 290-key catalogs verbatim

## Run

```sh
cp .env.example .env.local     # already created for you
npm install
npm run dev                    # http://localhost:5173
```

`npm run build` · `npm run preview` · `npm run typecheck`.

The Vite dev server proxies `/apis/*` (incl. WebSocket upgrades) to the gateway
at `http://localhost:18080` — start the backend with `deploy/docker-compose.yml`
for live data. **With the backend offline the app automatically serves mock
data** and shows an honest "Offline · sample data" banner.

### Environment (`.env.local`)

| var | meaning |
|---|---|
| `VITE_API_BASE` | API path/URL (dev: `/apis`, proxied) |
| `VITE_GATEWAY_PROXY` | where the dev proxy forwards `/apis` |
| `VITE_USE_MOCK` | `1` forces mocks; `0` calls the API, mocks only on failure |
| `VITE_DEV_ACCOUNT_ID` | dev bearer = account UUID (seeds a signed-in member) |

## Architecture

```
src/
  types/        backend DTO types (exact json shapes: cents, UPPERCASE enums, ISO dates)
  services/     centralized axios client + per-domain service fns, all behind withFallback
  mock/         stateful in-memory backend matching the DTO schemas (offline path)
  hooks/        TanStack Query hooks (queries + mutations) · session · timers · dutch engine
  i18n/         provider + the 4 reused catalogs + dir/RTL handling
  navigation/   router (animated stack) + bottom nav
  components/   ui/ design-system primitives (Icon, Seal, Chip, Money, ProductArt, Sheet…) + LotCard
  pages/        one screen per route
  lib/          formatting (cents→$), lot enrichment (category/accent), view models
  styles/       theme.css (ported design system) + app.css (mobile shell)
```

### Data flow — real API, offline-resilient

Every service call is `withFallback(() => http…, () => mock…)`:

1. `VITE_USE_MOCK=1` → always mock.
2. else call the gateway; on **backend-unavailable** (network error / 5xx) →
   serve the mock and flip the offline banner on.
3. **Genuine business errors (4xx)** — "out of credits", "invalid invite" — are
   re-thrown so the UI shows the real failure instead of faking success.

### Screen → endpoint map

| Screen | Service / endpoint |
|---|---|
| Gallery (`/`) | `GET /apis/gallery/weekly` |
| Lot detail (`/lot/:id`) | `GET /apis/lots/{id}` |
| Dutch live (`/auction/:id`) | `GET /apis/auctions/{id}` + price engine, `POST …/reserve\|lock\|buy` |
| Passive (`/passive/:id`) | `GET …/standing`, `POST …/bid` (Vickrey + UniqBid) |
| Bid store (`/bidstore`) | `GET /apis/bids/packages\|wallet`, `POST /apis/bids/buy` |
| Escrow (`/escrow/:id`) | `GET /apis/escrow/{id}`, `POST …/fund\|confirm` |
| Vault (`/vault`) | `GET /apis/vault`, `POST /apis/vault/objects/{id}/list\|buyback` |
| Invite / KYC | `POST /apis/invites/redeem`, `POST /apis/kyc/start\|verify`, `GET /apis/kyc/status` |
| Membership / Account | `GET /apis/me` |

### Conventions kept from the system contract

- **Money is `int64` USDC cents** end-to-end; converted to a display string only
  at the render edge (`lib/format.ts`). **Bid credits are whole units** ($1 each).
- **API is language-neutral** — lot titles/descriptions arrive as plain strings;
  only UI chrome is translated. Category, accent, and product art are
  **client-owned presentation** derived in `lib/enrich.ts`.
- Enums are `MONOSPACE_UPPERCASE`; dates are ISO-8601 UTC.
- The Dutch price is **server-authoritative**: the client engine animates between
  reads from the server's params; `buy` is re-validated server-side.

### Integration note

The gallery only knows lot ids, so the client opens auctions/escrow by lot id —
the mock treats `lotId === auctionId === tradeId`. A real integration would carry
an explicit `auctionId` on the lot DTO; that assumption is isolated to the
`services/` + `mock/` layers and nothing in the UI depends on it.
