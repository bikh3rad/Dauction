# Dauction — handoff

Three files for building **Dauction** (luxury auction platform, GCC) as a **Go microservices**
backend + **React** frontend, with the prototype in this project as the UI spec.

| File | What it is |
|---|---|
| `CLAUDE.md` | Root project memory — service topology, auction state machines (Dutch / Vickrey / UniqBid), escrow funds-conservation invariant, the bid-credit economy, conventions. Put it at the repo root. |
| `BUILD_PROMPT.md` | Paste into Claude Code to kick off the build: Phase 0 (repo + contracts + GitHub push) then one agent per microservice, then the React frontend that applies the prototype UI. |
| `AGENTS_AND_GITHUB.md` | This file — how to run one agent per microservice, and the exact GitHub steps for `bikh3rad/Dauction`. |

---

## One Claude Code agent per microservice

The architecture is a **monorepo of independent Go services** (each a `mequq/go-template` clone),
integrated only through `proto/` event + DTO contracts — never a shared database. That boundary is
what lets one agent own one service without stepping on another.

**Workflow**

1. **You (or a "platform" agent) do Phase 0 alone:** scaffold the monorepo, author `proto/` (event
   names + DTOs from `CLAUDE.md`), write `deploy/docker-compose.yml`, push `main`. Freeze `proto/`.
2. **Spawn one Claude Code session per service** (`gateway, identity, invite, kyc, vault, catalog,
   auction-dutch, auction-passive, bids, escrow, dispute, notifier`). For each session:
   - Open it scoped to `services/<name>/`.
   - Give it the root `CLAUDE.md` + a 10-line service note (its tables, states, events in/out, routes).
   - Instruct: *stay in your folder; never edit siblings; integrate only via `proto/` events and the
     owner service's API; run `make generate` and show the `wire_gen.go` diff; land one PR.*
3. **Order matters by dependency, not by importance:** `identity/invite/kyc` → `vault/catalog` →
   `bids` → `auction-dutch` → `auction-passive` → `escrow/dispute` → `notifier/gateway` → `web/`.
   Earlier services can be stubbed via their `proto/` contract so agents work in parallel.
4. **Merge gate:** a service merges only when its Definition of Done (CLAUDE.md §10) is green.

**Tips**
- Keep each agent's context small: one service folder + `proto/`. Don't hand it the whole monorepo.
- The funds-conservation test (escrow) and the Vickrey/UniqBid winner-rule tests (auction-passive)
  are required — they're the riskiest logic and the cheapest to verify.
- The frontend agent comes last and treats the HTML prototype as the visual + interaction spec.

---

## Push to GitHub — `git@github.com:bikh3rad/Dauction.git`

From the monorepo root, after Phase 0 scaffolding:

```sh
git init
git add .
git commit -m "chore: scaffold Dauction monorepo + proto contracts"
git branch -M main

# SSH (preferred — needs your SSH key added to GitHub):
git remote add origin git@github.com:bikh3rad/Dauction.git
git push -u origin main
```

If the repo doesn't exist yet, create it first with the GitHub CLI (also handles auth):

```sh
gh auth login
gh repo create bikh3rad/Dauction --public --source=. --remote=origin --push
```

If you prefer HTTPS over SSH:

```sh
git remote add origin https://github.com/bikh3rad/Dauction.git
git push -u origin main      # use a Personal Access Token when prompted for a password
```

**After that**, each microservice agent should branch and PR:

```sh
git checkout -b feat/<service>
# ... build the service ...
git add services/<service> && git commit -m "feat(<service>): vertical slice + wiring"
git push -u origin feat/<service>
gh pr create --fill            # or open the PR on github.com
```

Add a `.gitignore` (Go + Node) and turn on branch protection for `main` so every service lands via
review. Keep `proto/` changes to dedicated PRs — it's the shared contract every agent depends on.
