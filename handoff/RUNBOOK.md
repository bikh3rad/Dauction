# Dauction — multi-agent command runbook (per level)

Exact commands for every level of a parallel, agent-per-service build. Conventions:
`$ROOT` = the main clone; `$WT` = `$ROOT/..` (where worktrees live). Replace `<svc>` with a service name.
Each **agent** = one `claude` session in one worktree. "Paste:" lines go into that session.

```sh
export ROOT=~/code/Dauction
export WT=~/code            # worktrees created as $WT/dauction-<svc>
```

────────────────────────────────────────────────────────
## LEVEL 0 — Foundation, contracts & i18n (one session, solo)
────────────────────────────────────────────────────────
```sh
mkdir -p $ROOT && cd $ROOT
git init && git branch -M main
cd $ROOT && claude
```
Paste:
> Execute `handoff/PHASE0.md` exactly. Scaffold the monorepo (`services/ web/ proto/ i18n/ deploy/`),
> move `handoff/CLAUDE.md` to root `CLAUDE.md`, author and FREEZE `proto/` (events §2 + DTOs §6) and
> `i18n/` (`keys.md`, `en/fa/ar/tr.json` with identical keys, `locales.json`, error-code→message table,
> key-parity check wired into `make check`), write `deploy/docker-compose.yml` (pg-per-service + NATS
> JetStream + Jaeger) and a root `Makefile`. Then STOP. (No root `go.work` — services share the
> module name `application`, which a single Go workspace cannot hold twice.)

Verify + push:
```sh
cd $ROOT
make check                            # i18n key-parity must pass
docker compose -f deploy/docker-compose.yml config -q   # compose is valid
git add . && git commit -m "chore: scaffold monorepo, proto + i18n contracts, infra"
git remote add origin https://github.com/bikh3rad/Dauction.git
git push -u origin main
# repo missing? gh repo create bikh3rad/Dauction --public --source=. --remote=origin --push
```
Gate: do not continue until `main` has frozen `proto/` + `i18n/`.

────────────────────────────────────────────────────────
## LEVEL 1 — create worktrees for the wave (one branch/folder per service)
────────────────────────────────────────────────────────
```sh
cd $ROOT
git fetch origin && git checkout main && git pull
# Wave A example — repeat the pattern for each service in the wave:
for s in identity invite kyc; do
  git worktree add $WT/dauction-$s -b feat/$s main
done
git worktree list                      # sanity: one path per service
```

────────────────────────────────────────────────────────
## LEVEL 2 — run the wave (N parallel agents)
────────────────────────────────────────────────────────
Open one terminal/tmux pane per service. In each:
```sh
cd $WT/dauction-<svc> && claude
```
Paste (per session):
> Read repo-root `CLAUDE.md` (binding) and `handoff/agents/<svc>.md` (your scope). You own ONLY
> `services/<svc>/` plus read-only `proto/` and `i18n/`. Build to the Definition of Done. Use the
> template's `placeholder` slice. Run `make generate` after any wire.go change; never edit wire_gen.go.
> Backend stays language-neutral — emit error CODES, never prose. Commit on this branch; do not touch
> other services or the shared contracts.

Per-service inner loop (the agent runs these; you watch):
```sh
cd $WT/dauction-<svc>/services/<svc>
cp config.example.yaml config.yaml      # first time
make devtools                           # first time
make generate                           # after every wire.go edit
make swagger
make check && go test ./...
go run ./cmd/app --config ./config.yaml # smoke a single service
```
Tmux helper to fan out a wave:
```sh
for s in identity invite kyc; do
  tmux new-window -n $s "cd $WT/dauction-$s && claude"
done
```

────────────────────────────────────────────────────────
## LEVEL 3 — land each service (one PR per service)
────────────────────────────────────────────────────────
```sh
cd $WT/dauction-<svc>
git add services/<svc> && git commit -m "feat(<svc>): vertical slice + wiring + tests"
git fetch origin && git rebase origin/main      # stay current
git push -u origin feat/<svc>
gh pr create --fill --base main                 # or open on github.com
# after CI green + review:
gh pr merge --squash --delete-branch
# free the worktree:
cd $ROOT && git worktree remove $WT/dauction-<svc>
```
CI must run: root i18n key-parity (`make check`), the service `make check`, `go test ./...`.

**There is intentionally NO root `go.work`.** Every service keeps the template's module name
`application`, and a Go workspace cannot contain two modules with the same module path — so a
shared workspace is impossible by construction. Each service builds/tests standalone from its own
folder (`cd services/<svc> && go build ./... && go test ./...`); the root `make check` / `make test`
loop over `services/*/` and do exactly that. A service PR stages ONLY `services/<svc>/`
(the `git add services/<svc>` above does exactly that) — nothing else to register after merge.

────────────────────────────────────────────────────────
## LEVEL 4 — advance waves (repeat 1→3 per wave, in order)
────────────────────────────────────────────────────────
```
Wave A  identity · invite · kyc
Wave B  vault · catalog
Wave C  bids
Wave D  auction-dutch
Wave E  auction-passive
Wave F  escrow · dispute
Wave G  notifier · gateway       # last backend wave
```
A later wave may start early by coding against an upstream service's `proto/` contract (stub it).
**Contract change?** Serialize it:
```sh
git worktree add $WT/dauction-proto -b chore/proto-<topic> main
# edit proto/ and/or i18n/ only, commit, PR, merge FIRST
# then every active agent: git fetch origin && git rebase origin/main
```

────────────────────────────────────────────────────────
## LEVEL 5 — integrate & smoke (after each wave)
────────────────────────────────────────────────────────
```sh
cd $ROOT && git checkout main && git pull
make up                                  # pg-per-service + NATS + Jaeger + all merged services
# happy-path smoke (adjust host/port to the gateway):
curl -s localhost:8080/apis/gallery/weekly | jq '.[].id'
curl -s -X POST localhost:8080/apis/invites/redeem -d '{"code":"LUX-7F2A-9KQ"}'
curl -s -X POST localhost:8080/apis/bids/buy        -d '{"packageId":"pkg-100"}'
curl -s -X POST localhost:8080/apis/auctions/lot-14/bid -d '{"price":237}'
# watch the event flow:
open http://localhost:16686            # Jaeger
make down
```

────────────────────────────────────────────────────────
## LEVEL 6 — frontend (web/), applies the prototype 1:1
────────────────────────────────────────────────────────
```sh
cd $ROOT && git worktree add $WT/dauction-web -b feat/web main
cd $WT/dauction-web && claude
```
Paste:
> Build `web/` per `handoff/MASTER_PROMPT.md` Phase F. Vite + React + TS + TanStack Query against the
> gateway; thin WS client for the Dutch price feed + passive countdowns. Lift tokens VERBATIM from
> `theme.css`; port EVERY key from `i18n/` into react-i18next for `en·fa·ar·tr` (dir from `locales.json`,
> RTL for fa·ar). Reproduce the prototype's mobile buyer app, desktop admin, and Flow view, including
> the Dutch live auction, Vickrey + UniqBid screens, bid store, escrow strip, and list-to-auction.
```sh
cd $WT/dauction-web/web
npm install && npm run dev               # iterate
npm run build && npm run lint
git add web && git commit -m "feat(web): React app applying the prototype, 4 languages"
git push -u origin feat/web && gh pr create --fill
```

────────────────────────────────────────────────────────
## Daily ops cheat-sheet
────────────────────────────────────────────────────────
```sh
git worktree list                         # what's checked out where
git fetch origin && git rebase origin/main  # in each active worktree, keep current
tmux ls                                   # your running agent sessions
make up / make down                       # local stack
make check                                # i18n parity + lint, repo-wide
```

**Invariants to never break, at any level:** one service = one folder = one PR; DB-per-service (events,
not shared tables); `proto/` + `i18n/` changed only via their own serialized PRs; backend returns codes
+ data, never localized prose; all four languages keep identical i18n keys (CI-enforced).
