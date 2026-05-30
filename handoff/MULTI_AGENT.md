# Running Dauction in multi-agent mode

**Multi-agent** = several **independent** Claude Code sessions running in parallel, each in its own
checkout, each owning one service, coordinating through git + the frozen contracts. (This is different
from **sub-agent** mode, where one orchestrator session spawns helpers inside itself. Here there is no
orchestrator — the contracts in `proto/` and `i18n/` are the coordination layer.)

Prereq: **Phase 0 is merged to `main`** (monorepo skeleton + frozen `proto/` + frozen `i18n/` + infra).
Nothing below works until the shared contracts exist.

---

## 1. Give each agent an isolated checkout (git worktrees)

Don't point five Claude Code sessions at one working directory — they'll clobber each other. Use **git
worktrees**: one folder + one branch per service, all sharing the same repo history.

```sh
cd Dauction
# one worktree per service you're about to staff:
git worktree add ../dauction-identity      -b feat/identity      main
git worktree add ../dauction-invite        -b feat/invite        main
git worktree add ../dauction-kyc           -b feat/kyc           main
git worktree add ../dauction-vault         -b feat/vault         main
git worktree add ../dauction-catalog       -b feat/catalog       main
git worktree add ../dauction-bids          -b feat/bids          main
git worktree add ../dauction-auction-dutch -b feat/auction-dutch main
# …one per service
```

Each `../dauction-<service>/` is a full, independent checkout on its own branch. An agent working there
can build, run `make generate`, and commit without touching anyone else's files.

> Monorepo Go tip: add a root `go.work` (`go work init ./services/*`) so each service stays its own
> module but they resolve locally. Treat `go.work`, `proto/`, `i18n/`, and `deploy/` as **shared,
> change-controlled** files (see §4).

---

## 2. Launch one Claude Code session per worktree

Open a separate terminal (or tmux pane) per service and start Claude Code **inside that worktree**:

```sh
cd ../dauction-identity && claude     # session 1
cd ../dauction-bids     && claude     # session 2
# …etc
```

Kickoff message for each session (paste its scoped prompt):

> Read the repo-root `CLAUDE.md` (binding) and `handoff/agents/<service>.md` (your scope). You own
> **only** `services/<service>/` plus read-only `proto/` and `i18n/`. Build it to the Definition of
> Done, commit on this branch, and open a PR. Do not edit other services or the shared contracts.

Each session sees the whole repo (so it can read `proto/`, `CLAUDE.md`, the prototype) but is
**instructed to write only inside its service folder**. That instruction + the worktree isolation is
what keeps agents out of each other's way.

---

## 3. Run agents in dependency waves (parallelism within a wave)

Spawn in waves; everything inside a wave runs **in parallel**. A later wave can start early by stubbing
an upstream service against its `proto/` contract (the contract is enough to code against).

```
Wave A  identity · invite · kyc
Wave B  vault · catalog
Wave C  bids
Wave D  auction-dutch
Wave E  auction-passive
Wave F  escrow · dispute
Wave G  notifier · gateway          (last: depend on everyone's contracts)
Wave H  web/  frontend              (applies the HTML prototype 1:1, all 4 languages)
```

Practical staffing: 2–4 sessions at once is comfortable to supervise. You don't need 12 terminals —
finish a wave, merge, then reassign worktrees to the next.

---

## 4. Coordination rules (the whole point)

- **Contracts are frozen.** `proto/` and `i18n/` are the integration surface. An agent **never** edits
  them to make its code compile. If a real contract gap appears, that change goes in a **dedicated PR**
  (its own short-lived `chore/proto-*` branch), reviewed and merged first; then dependent agents
  `git fetch && git rebase origin/main` to pick it up. Serialize contract changes — never parallel.
- **DB-per-service, no shared tables.** Cross-service data flows via NATS events or the owner's API.
  An agent that wants someone else's data adds a dependency to `CLAUDE.md §2`, it doesn't reach in.
- **One service = one PR.** Branch `feat/<service>`. CI must be green (the i18n key-parity check + the
  service's `make check` + `go test ./...`). Rebase on `main` before merge.
- **i18n discipline.** Services stay language-neutral (codes + data only). Any new user-facing string
  is a **key added to all four catalogs** (`en/fa/ar/tr`) in an `i18n/` PR — the key-parity check fails
  a PR that adds a key to only one language. The `web/` agent consumes the catalogs; backend agents
  only emit **error codes**, never prose.

---

## 5. Integrate & verify

- Bring the system up locally with `make up` (`deploy/docker-compose.yml`: pg-per-service + NATS +
  Jaeger). Each merged service registers behind the `gateway`.
- Smoke the cross-service paths after each wave: redeem invite → KYC approve → browse gallery →
  reserve/lock/buy (Dutch) and buy-credits → bid → close/resolve (Vickrey/UniqBid) → escrow
  fund/confirm/release. Watch the event flow in Jaeger.
- The `web/` agent runs last and is validated against the prototype in all four languages (RTL for
  fa·ar), including the live Dutch auction, the two passive auctions, the bid store, and the Flow view.

---

## 6. When you're done with a worktree

```sh
# after the PR is merged:
git worktree remove ../dauction-identity
git branch -d feat/identity
```

---

### TL;DR
Merge Phase 0 → `git worktree add` one folder/branch per service → one `claude` session per worktree,
each handed `CLAUDE.md` + its `handoff/agents/<service>.md` → run in dependency waves, parallel within a
wave → integrate only through frozen `proto/` + `i18n/` → one PR per service, CI green, rebase, merge.
