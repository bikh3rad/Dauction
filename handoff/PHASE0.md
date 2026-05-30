# Phase 0 — Foundation, contracts & internationalization (do this alone, then PAUSE)

The bootstrap an agent (or you) runs before any service is built. Output: a pushed monorepo, a frozen
`proto/` contract, a frozen **i18n contract**, and local infra. Stop for review at the end.

Repo root `CLAUDE.md` is binding. Goal of Phase 0 is the *shared surface* every later agent depends on:
**proto events/DTOs** and the **i18n message catalog** — get them right and freeze them.

---

## 0.1 Monorepo skeleton

```
Dauction/
  services/            # one go-template clone per service (created in later phases)
  web/                 # React app (built in Phase F)
  proto/               # shared event + DTO contracts  ← author now, freeze
  i18n/                # shared message catalog + locale policy  ← author now, freeze
  deploy/              # docker-compose (pg-per-service + NATS + Jaeger)
  CLAUDE.md  README.md  .gitignore
```

Move `handoff/CLAUDE.md` → repo-root `CLAUDE.md`. Add `.gitignore` (Go + Node).

---

## 0.2 `proto/` — the event + DTO contract (freeze before services start)

Author the event names from `CLAUDE.md` §2 and the request/response DTOs from §6 as `.proto`
(or typed JSON schema if you prefer REST-only). Rules baked into the contract:

- **Money** is `int64` USDC **cents**; **bid_credit** is `int64` credits. Distinct types — never share a field.
- **Timestamps** are ISO-8601 **UTC** strings (or epoch millis), never localized.
- **State** is a `MONOSPACE_UPPERCASE` enum string; it is protocol vocabulary, **never translated**.
- Events carry an **idempotency key** and a producer + occurred_at; cross-service writes use an outbox.
- Generate stubs into each service later; `proto/` itself has no business logic.

---

## 0.3 i18n contract — **internationalization is a Phase-0 decision**

The platform speaks **four languages: `en`, `fa`, `ar`, `tr`**. Lock the policy now so the API stays
language-neutral and the client owns all copy.

**Server side (all services):**
- The API returns **codes and data, never prose**: enum codes (`OPEN`, `FULL_LOCKED`, `DISPUTED`),
  integer amounts, ISO-8601 UTC times, IDs. No human sentences in responses.
- **Errors** return a stable machine `code` (e.g. `OUT_OF_CREDITS`, `KYC_REQUIRED`, `INVALID_TRANSITION`)
  via `dto.HandleError`; the **client** maps each code to localized text. Do **not** localize in Go.
- If an entity has owner-authored multilingual content (e.g. a lot **title/description** in en·fa·ar·tr),
  store it as a **JSONB map** `{ "en": "...", "fa": "...", "ar": "...", "tr": "..." }` and return the
  whole map; the client picks the active language (fallback → `en`). Define this shape in `proto/`.
- Persist a user's **preferred locale** on the `account` (one of `en|fa|ar|tr`) so the gateway can hint
  the client and so notifications/emails (if any) pick a language — but the API payloads stay neutral.
- Numbers: never pre-format on the server. Send raw integers; the client formats with `Intl` (tabular,
  thousands separators, ` USDC` suffix, Eastern-Arabic digits where the locale calls for it).

**Shared catalog (`i18n/`):**
- Create `i18n/keys.md` — the canonical key list, seeded by porting **every key** from the prototype's
  `i18n.js` (UI strings) plus an **error-code → message** table for the codes above.
- Provide the four catalogs as JSON: `i18n/en.json`, `i18n/fa.json`, `i18n/ar.json`, `i18n/tr.json`,
  each containing the same keys. These are consumed verbatim by `web/` (react-i18next) in Phase F.
- `i18n/locales.json` — the locale policy the client reads:
  ```json
  {
    "default": "en",
    "supported": ["en", "fa", "ar", "tr"],
    "dir": { "en": "ltr", "fa": "rtl", "ar": "rtl", "tr": "ltr" },
    "label": { "en": "EN", "fa": "فا", "ar": "ع", "tr": "TR" },
    "fonts": { "ltr": "IBM Plex Sans", "rtl": "Vazirmatn" },
    "numerals": { "en": "latn", "tr": "latn", "fa": "arabext", "ar": "arab" }
  }
  ```
- **RTL** (`fa`, `ar`) flips layout direction; **LTR** (`en`, `tr`). State codes always render in
  `IBM Plex Mono`, uppercase, **unflipped/untranslated** even in RTL.
- A simple **CI check** (script) asserts all four JSON files have an identical key set — no missing
  translations. Wire it into `make check` at the root.

> Net: Phase 0 fixes *where* language lives (client) and *what* is shared (keys + locale policy +
> multilingual JSONB shape). No service ever returns localized prose.

---

## 0.4 Local infra — `deploy/docker-compose.yml`

One Postgres per service (isolated DBs), **NATS with JetStream**, and **Jaeger** (OTLP). Each service
later points its koanf config at its own DB + the shared NATS + the Jaeger collector. Add a root
`make up` / `make down` convenience.

---

## 0.5 Root tooling

- Root `Makefile`: `up`, `down`, and `check` (runs the i18n key-parity check + `go vet`/lint across
  `services/*` once they exist).
- `README.md`: one-paragraph product summary, the service map (link `CLAUDE.md`), and "how to run a
  service agent" (link `handoff/agents/`).

---

## 0.6 Push to GitHub

```sh
git init && git add . && git commit -m "chore: scaffold Dauction monorepo, proto + i18n contracts, infra"
git branch -M main
git remote add origin https://github.com/bikh3rad/Dauction.git
git push -u origin main
# or, if the repo doesn't exist yet:
# gh repo create bikh3rad/Dauction --public --source=. --remote=origin --push
```

Turn on branch protection for `main`; keep `proto/` and `i18n/` changes to dedicated PRs — they're the
contracts every agent depends on.

---

## 0.7 Definition of done for Phase 0 — then STOP

- [ ] Monorepo skeleton committed; `CLAUDE.md` at root; `.gitignore` (Go + Node).
- [ ] `proto/` authored & frozen — events (§2) + DTOs (§6), int64 money/credits, UTC times, enum states,
      idempotency keys, multilingual-JSONB shape defined.
- [ ] `i18n/` authored & frozen — `keys.md`, `en/fa/ar/tr.json` (identical key sets), `locales.json`
      (default/supported/dir/label/fonts/numerals), error-code→message table; key-parity check in `make check`.
- [ ] `deploy/docker-compose.yml` brings up pg-per-service + NATS(JetStream) + Jaeger; `make up` works.
- [ ] Pushed to `https://github.com/bikh3rad/Dauction` (`main`), branch protection on.
- [ ] **Pause for review** before spawning service agents.
