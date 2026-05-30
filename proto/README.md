# `proto/` — the shared contract (FROZEN)

This directory is the **only shared surface** between Dauction services. It defines the
async event contract (NATS/JetStream) and the request/response DTOs for the sync APIs.
Services integrate **only** through these — never DB → DB (CLAUDE.md §0, §9).

> **Freeze policy.** `proto/` and `i18n/` changes land via **dedicated PRs** only.
> Every service agent depends on this contract; treat additions as additive (new fields
> with new tags, new enum values, new event arms) and avoid breaking changes.

## Layout

```
proto/dauction/
  common/v1/common.proto    # Money, BidCredit, I18nText, Locale, state enums, ErrorCode
  events/v1/events.proto    # EventEnvelope + every domain event (CLAUDE.md §2)
  identity|invite|kyc|vault|catalog|auction|bids|escrow|dispute /v1/*.proto  # DTOs (§6)
```

## Contract rules baked in

- **Money is `int64` USDC cents** (`common.Money`); **bid credits are `int64`** (`common.BidCredit`).
  They are **distinct message types** so they can never share a field. No floats for money, ever.
- **Timestamps are ISO-8601 UTC strings** (field names end `_at`), never localized.
- **State is a `MONOSPACE_UPPERCASE` enum** — the enum *value name* is the wire string
  (`OPEN`, `FULL_LOCKED`, …). It is protocol vocabulary and is **never translated**.
- **Events** carry an `EventEnvelope` with `idempotency_key`, `producer`, and `occurred_at`.
  Cross-service writes use an **outbox** keyed by `idempotency_key`.
- **Errors** return a stable `ErrorCode` (machine code) via `dto.HandleError`; the **client**
  maps each code to localized text using the `err.<CODE>` keys in `i18n/`. No prose in Go.
- **Multilingual owner content** uses `common.I18nText` `{ en, fa, ar, tr }` (JSONB), returned
  whole; the client picks the active language and falls back to `en`.

## Codegen (later phases)

`proto/` has **no business logic**. Each service generates stubs into its own module:

```sh
cd proto && buf generate              # -> proto/gen/... (or per-service out)
buf lint && buf breaking --against '.git#branch=main'
```

`buf.gen.yaml` uses managed mode to map the canonical `go_package` prefix; a service may
override the output path when generating into `services/<name>/internal/gen/`.
