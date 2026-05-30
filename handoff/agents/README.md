# Per-service agent prompts

Run ONE Claude Code session per file below, each scoped to `services/<name>/` + read-only `proto/`.
Do Phase 0 from `../MASTER_PROMPT.md` first (scaffold monorepo, author + freeze `proto/`, push to
GitHub). Then spawn agents in dependency order; earlier services can be stubbed via their proto contract.

Order:
1. identity, invite, kyc
2. vault, catalog
3. bids
4. auction-dutch
5. auction-passive
6. escrow, dispute
7. notifier, gateway   (last — depend on everyone's contracts)
8. web/ frontend       (see ../MASTER_PROMPT.md Phase F — applies the HTML prototype 1:1)

Each agent: stay in your folder, integrate only via `proto/` events + the owner service's API, never
another service's DB. Land one PR per service; merge only when its Definition of Done is green.

| Order | File |
|---|---|
| 1 | identity.md · invite.md · kyc.md |
| 2 | vault.md · catalog.md |
| 3 | bids.md |
| 4 | auction-dutch.md |
| 5 | auction-passive.md |
| 6 | escrow.md · dispute.md |
| 7 | notifier.md · gateway.md |
