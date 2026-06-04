# Dauction — API Gateway

The **edge** of the Dauction platform: the single public entrypoint that
reverse-proxies every `/apis/*` route to the owning backend service after running
authN, a tier/KYC guard and rate-limiting. It owns no domain data (stateless: no
Postgres / NATS / Redis). Go module name: `application`.

See the repo-root `CLAUDE.md` for the binding system contract (topology §2,
routes §6). This README documents the gateway's own surface.

## Responsibilities

1. **Reverse proxy** — longest-prefix route table → upstream service base URL.
2. **AuthN** — resolve the caller and inject a trusted `X-Account-Id` downstream.
3. **Tier/KYC guard** — gate participation routes on `MEMBER/VIP + KYC APPROVED`.
4. **Rate-limit** — fixed-window per account/IP.
5. **Health + docs** — `/healthz/*`, `/metrics`, `/swagger/`.

## Route table → upstream

Mounted under `/apis`. Matched by HTTP method + longest path prefix, with
suffix/exact constraints taking precedence (`internal/router/routes.go`).

| Route (method + path) | Upstream | Access |
|---|---|---|
| `GET /apis/gallery/...`, `GET /apis/gallery` | `catalog` | public |
| `GET /apis/lots/...` | `catalog` | public |
| `POST /apis/invites/redeem` | `invite` | public |
| `POST /apis/kyc/start`, `POST /apis/kyc/verify` | `kyc` | public |
| `GET /apis/me` | `identity` | authed |
| `/apis/internal/accounts/...`, `/apis/admin/accounts/...` | `identity` | authed |
| `/apis/admin/invites/...`, `/apis/invites/...` | `invite` | authed |
| `/apis/admin/kyc...`, `/apis/kyc/...` | `kyc` | authed |
| `/apis/vault`, `/apis/vault/...` | `vault` | participate |
| `/apis/bids/...` | `bids` | authed |
| `POST /apis/auctions/{id}/reserve\|lock\|buy` | `auction-dutch` | participate |
| `POST/GET /apis/auctions/{id}/bid\|standing` | `auction-passive` | participate |
| `/apis/admin/auctions/{id}/open\|complete\|abort` | `auction-dutch` | authed |
| `/apis/admin/auctions/{id}/close` | `auction-passive` | authed |
| `/apis/escrow/{id}/dispute/resolve` | `dispute` | authed |
| `/apis/admin/escrow/...` | `escrow` | authed |
| `/apis/escrow/...` | `escrow` | participate |
| `/apis/live/...` (WebSocket/SSE) | `notifier` | authed |
| anything else under `/apis/` | — | `404 NOT_FOUND` |

- **public** — no auth, no guard.
- **authed** — a valid bearer is required, but no tier/KYC level.
- **participate** — `tier ∈ {MEMBER, VIP}` **and** `kyc = APPROVED` (root §1).

### Dutch vs Passive (shared prefix)

Both auction engines live under `/apis/auctions/{id}/`. The gateway disambiguates
by the **action suffix**: `reserve|lock|buy` → `auction-dutch`,
`bid|standing` → `auction-passive`. A cleaner long-term scheme is
`/apis/auctions/dutch/...` vs `/apis/auctions/passive/...`, but the current routes
match the frozen list in root `CLAUDE.md` §6 by suffix.

## Middleware chain

Applied to the `/apis/` mount (outermost → innermost):

1. **Request-ID + structured access logging** (`SetRequestContextLogger` + httplogger).
2. **Panic recovery** (`pkg/middlewares.RecoveryMiddleware`; the server also wraps
   a second recovery via `app.NewRecoveryMiddleware`).
3. **Auth** — strips any inbound `X-Account-Id` / `X-Account-Tier` /
   `X-Kyc-Approved`, parses the bearer, injects the trusted `X-Account-Id`.
4. **Rate-limit** — fixed-window, keyed by the resolved account (else client IP).
   Over limit → `429 RATE_LIMITED`. Configurable; default 100 req / 10s.
5. **Route match → tier/KYC guard → trusted-header injection → reverse-proxy.**

## Authenticating in dev

Dev scheme (documented; real JWT validation is future work):

```
Authorization: Bearer <accountId>
```

The bearer token IS the account UUID. The gateway resolves it, **strips any
client-supplied identity headers**, then injects the trusted ones downstream:

- `X-Account-Id` — always (when authenticated).
- `X-Account-Tier` — on gated routes (`GUEST|MEMBER|VIP`).
- `X-Kyc-Approved` — on gated routes (`true|false`).

Example:

```sh
curl -H "Authorization: Bearer 11111111-1111-1111-1111-111111111111" \
     http://localhost:8080/apis/bids/wallet
```

## Tier/KYC guard read model

The guard reads identity's projection:

```
GET {APP_ACCESS_BASEURL}/apis/internal/accounts/{id}/access
→ { "id", "tier", "kycStatus", "eligible" }
```

via the `RepositoryAccess` seam (`repo.identityAccess`), cached ~5s. Decisions:
public → allow; participation → require `MEMBER/VIP + APPROVED` else
`TIER_REQUIRED` / `KYC_REQUIRED` (403); unauthenticated on a non-public route →
`UNAUTHORIZED` (401).

## Error codes (language-neutral)

`{ "code", "message", "details" }` — the React client localizes `code`:

`OK` · `NOT_FOUND` · `RESOURCE_INVALID` · `UNAUTHORIZED` · `TIER_REQUIRED` ·
`KYC_REQUIRED` · `RATE_LIMITED` · `UPSTREAM_UNAVAILABLE` · `INTERNAL`.

## Config

`config.example.yaml` → copy to `config.yaml`. Key env overrides (`APP_`-prefixed):

| Env | Default | Meaning |
|---|---|---|
| `APP_SERVER_HTTP_ADDR` | `:8080` | listen address |
| `APP_UPSTREAMS_IDENTITY` … `_NOTIFIER` | `http://<svc>:8080` | upstream URLs |
| `APP_ACCESS_BASEURL` | `http://identity:8080` | guard read-model base |
| `APP_ACCESS_TIMEOUTSECONDS` | `2` | guard HTTP timeout |
| `APP_RATELIMIT_ENABLED` | `true` | toggle limiter |
| `APP_RATELIMIT_LIMIT` | `100` | requests per window |
| `APP_RATELIMIT_WINDOWSECONDS` | `10` | window length |
| `APP_COLLECTOR_EXPORTERS_OTLP_ENDPOINT` | `jaeger:4317` | OTLP endpoint |

## Build & test

```sh
make generate   # regenerate Wire DI (after any wire.go edit)
make swagger     # regenerate ./docs
make check       # lint
go test ./...    # unit tests
make build       # container (runtime: gcr.io/distroless/static-debian12:nonroot)
```

## Layout

```
internal/
  router/    # route table → upstream + access requirement
  proxy/     # httputil.ReverseProxy pool (WS-upgrade aware)
  biz/       # access usecase (guard decision + cache) + RepositoryAccess seam
  repo/      # identityAccess: HTTP client to identity's read model
  entity/    # Access projection
  service/handler/  # ProxyHandler (the edge) + healthz
pkg/middlewares/    # auth (strip+inject), ratelimit, logging, recovery
app/        # upstreams + ratelimit config, lifecycle, OTel, koanf
```
