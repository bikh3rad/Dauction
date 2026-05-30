# i18n catalog — canonical keys (FROZEN)

The canonical message-key list for Dauction. **All UI copy and `LTR/RTL` live in the client**;
the API is language-neutral (enum codes, integer amounts, ISO-8601 UTC times). See CLAUDE.md §0.7
and `proto/` for the server contract.

> **Freeze policy.** `i18n/` and `proto/` changes land via **dedicated PRs** only — every agent
> depends on them. Adding a UI string means adding the key to **all four** catalogs (the
> `make check` key-parity gate enforces this). Adding an API error means adding both the
> `dauction.common.v1.ErrorCode` enum value **and** the `err.<CODE>` key in all four catalogs.

## Files

| File | Purpose |
|---|---|
| `en.json` `fa.json` `ar.json` `tr.json` | The four catalogs — **identical key sets**. Consumed verbatim by `web/` (react-i18next). |
| `locales.json` | Locale policy the client reads: default, supported, `dir`, `label`, `fonts`, `numerals`. |
| `keys.md` | This document — the canonical key list + error-code → message table. |
| `build_catalogs.mjs` | Regenerates the four JSON catalogs: UI keys ported verbatim from `../i18n.js` + the `err` table. |
| `check_keys.mjs` | CI key-parity check (wired into root `make check`). Fails on any missing/stray key. |

## Catalog shape

Each catalog is a flat map of **253 UI keys** (ported 1:1 from the prototype `i18n.js`) plus one
nested **`err`** object holding **36 error-code messages**. `en` is the fallback language.

```json
{
  "brand": "DAUCTION",
  "auc_current": "Current price",
  "...": "… 253 UI keys total …",
  "err": { "OUT_OF_CREDITS": "You're out of bid credits.", "...": "… 36 codes …" }
}
```

## Rendering rules (from `locales.json`)

- **Direction**: `fa`, `ar` = **RTL** (flip layout); `en`, `tr` = **LTR**.
- **Fonts**: LTR → `IBM Plex Sans`; RTL → `Vazirmatn`. Body serif elsewhere per the prototype.
- **Numerals**: `en`/`tr` → `latn`; `fa` → `arabext`; `ar` → `arab`. Format with `Intl` on the client.
- **State codes** (`OPEN`, `FULL_LOCKED`, `DISPUTED`, …) always render in **`IBM Plex Mono`,
  uppercase, unflipped and untranslated** — even in RTL. They are protocol vocabulary, not copy.
  (The `st_*` keys below are display labels used by the prototype's flow view; the wire states
  themselves come from `proto/` enums and are never translated.)
- **Amounts**: the server sends raw `int64` (USDC cents / bid credits); the client formats with
  `Intl` — tabular figures, thousands separators, ` USDC` suffix, Eastern-Arabic digits per locale.

---

## UI keys (253) — by section

### Brand & taglines — 3 keys

`brand` · `sub_tagline` · `tagline`

### Navigation — 6 keys

`nav_account` · `nav_activity` · `nav_bids` · `nav_closet` · `nav_gallery` · `nav_membership`

### Invitation — 9 keys

`inv_body` · `inv_cta` · `inv_demo` · `inv_err` · `inv_kicker` · `inv_ok` · `inv_ph` · `inv_req` · `inv_title`

### KYC / identity — 11 keys

`kyc_body` · `kyc_doc` · `kyc_doc_hint` · `kyc_kicker` · `kyc_otp` · `kyc_pending` · `kyc_phone` · `kyc_resend` · `kyc_send_otp` · `kyc_submit` · `kyc_title`

### Gallery — 11 keys

`gal_all` · `gal_ends` · `gal_floor` · `gal_kicker` · `gal_live` · `gal_lot` · `gal_start` · `gal_supply` · `gal_title` · `gal_upcoming` · `gal_watching`

### Lot detail — 14 keys

`lot_about` · `lot_authentic` · `lot_box` · `lot_brand` · `lot_cert` · `lot_condition` · `lot_docs` · `lot_enter` · `lot_inspected` · `lot_provenance` · `lot_ref` · `lot_starts` · `lot_view360` · `lot_year`

### Auction · live (Dutch) — 18 keys

`auc_buy` · `auc_buy_short` · `auc_current` · `auc_dropping` · `auc_floor` · `auc_live` · `auc_lock_cta` · `auc_locked` · `auc_need_lock` · `auc_next` · `auc_participants` · `auc_premium` · `auc_secs` · `auc_sold` · `auc_step` · `auc_watching` · `auc_won` · `auc_won_by`

### Escrow — 19 keys

`esc_balance` · `esc_complete_body` · `esc_complete_title` · `esc_deposit` · `esc_flow` · `esc_fund` · `esc_funded` · `esc_hammer` · `esc_lock` · `esc_lock_body` · `esc_lock_title` · `esc_locked` · `esc_release` · `esc_release_body` · `esc_release_title` · `esc_released` · `esc_remaining` · `esc_title` · `esc_total`

### Vault / closet — 14 keys

`clo_add` · `clo_address` · `clo_buyback` · `clo_buyback_body` · `clo_cash` · `clo_credit` · `clo_creditopt` · `clo_inauction` · `clo_incloset` · `clo_list` · `clo_sold` · `clo_sub` · `clo_title` · `clo_value`

### Membership & tiers — 12 keys

`mem_access` · `mem_current` · `mem_fee` · `mem_guest` · `mem_guest_a` · `mem_standard` · `mem_std_a` · `mem_sub` · `mem_title` · `mem_upgrade` · `mem_vip` · `mem_vip_a`

### State labels (display only — wire states are protocol enums) — 9 keys

`st_appraising` · `st_completed` · `st_delivered` · `st_disputed` · `st_funded` · `st_in_closet` · `st_in_transit` · `st_live` · `st_proposed`

### Common controls — 6 keys

`common_back` · `common_cancel` · `common_close` · `common_confirm` · `common_continue` · `common_lots`

### View switcher — 8 keys

`desk_nav_account` · `desk_wallet` · `switch_view` · `view_admin` · `view_buyer` · `view_desktop` · `view_flow` · `view_mobile`

### Admin (house operations) — 23 keys

`adm_action` · `adm_active_invites` · `adm_approve` · `adm_certify` · `adm_chain` · `adm_closet` · `adm_dispute` · `adm_escrow` · `adm_escrow_locked` · `adm_hold` · `adm_invites` · `adm_issued_by` · `adm_lots_week` · `adm_member` · `adm_object` · `adm_overview` · `adm_pending_kyc` · `adm_reject` · `adm_resolve` · `adm_revoke` · `adm_status` · `adm_title` · `adm_value`

### Access & participation — 20 keys

`await_live` · `enter_invite` · `full_locked` · `guest_banner` · `guest_tag` · `invite_elevate` · `lockfull_body` · `lockfull_cta` · `lockfull_title` · `member_tag` · `need_member` · `registered` · `req_body` · `req_cta` · `req_deposit` · `req_participate` · `req_title` · `reserved` · `tier_up` · `you_registered`

### Flow view — 17 keys

`flow_escrow_state` · `flow_legend` · `flow_open` · `flow_phase_access` · `flow_phase_discover` · `flow_phase_live` · `flow_phase_release` · `flow_phase_reserve` · `flow_phase_settle` · `flow_phase_verify` · `flow_sub` · `flow_title` · `role_buyer` · `role_escrow` · `role_house` · `role_inspector` · `role_seller`

### Auction modes & passive auctions — 26 keys

`auc_mode` · `bids_placed` · `closes` · `d_short` · `ends_in` · `enter_amount` · `h_short` · `lowest_unique_now` · `m_short` · `mode_dutch` · `mode_uniqbid` · `mode_vickrey` · `passive_kicker` · `place_for_1` · `sealed_until` · `standing` · `status_taken` · `status_unique` · `submit_sealed` · `uniqbid_rule` · `update_bid` · `vickrey_rule` · `you_lead` · `you_trail` · `your_bids` · `your_sealed_bid`

### Bid-credit economy — 15 keys

`best_value` · `bid_credits` · `bid_store` · `bid_store_sub` · `bid_wallet` · `bought_bids` · `buy_bids` · `credit` · `credits` · `need_bids` · `need_bids_cta` · `per_bid` · `pkg_bids` · `pkg_buy` · `pkg_save`

### List-to-auction — 12 keys

`dur_2d` · `dur_5d` · `dur_7d` · `list_choose_type` · `list_confirm` · `list_duration` · `list_floor` · `list_sub` · `list_submitted` · `list_title` · `set_by_owner` · `won_passive`

---

## Error-code → message table (36)

The API returns a stable machine `code` (`dauction.common.v1.ErrorCode`) via `dto.HandleError`;
the client maps it to localized text via the `err.<CODE>` key. **Never localize in Go.** The `en`
column below is the source of truth; `fa`/`ar`/`tr` live in the respective catalogs.

| Code | `en` message |
|---|---|
| `ACCESS_DENIED` | You don’t have access to this. |
| `ALREADY_FUNDED` | This purchase is already funded. |
| `AUCTION_CLOSED` | This auction has closed. |
| `AUCTION_NOT_OPEN` | This auction isn’t open yet. |
| `BID_TOO_LOW` | Your bid is below the reserve floor. |
| `DEPOSIT_NOT_LOCKED` | Lock your deposit to continue. |
| `DISPUTE_IN_PROGRESS` | A dispute is in progress. Release is on hold. |
| `DUPLICATE_BID` | You’ve already placed your sealed bid. |
| `ESCROW_FORFEITED` | The deposit was forfeited. |
| `FULL_LOCK_REQUIRED` | Lock the full amount to enter the room. |
| `FUNDING_WINDOW_EXPIRED` | The 24-hour funding window has passed. |
| `INSUFFICIENT_FUNDS` | Insufficient wallet balance. |
| `INTERNAL_ERROR` | Something went wrong on our end. Please try again. |
| `INVALID_TRANSITION` | That step isn’t available yet. |
| `INVITE_ALREADY_REDEEMED` | This invitation has already been redeemed. |
| `INVITE_INVALID` | Invalid or expired code. Each invitation admits one member. |
| `INVITE_REVOKED` | This invitation has been revoked. |
| `KYC_PENDING` | Your verification is under review. Guest access until approved. |
| `KYC_REJECTED` | Your verification was not approved. Please resubmit. |
| `KYC_REQUIRED` | Verify your identity to participate. |
| `LOT_NOT_CERTIFIED` | This lot is still awaiting certification. |
| `NOT_ESCROW_PARTICIPANT` | You’re not a party to this escrow. |
| `OTP_EXPIRED` | That code has expired. Request a new one. |
| `OTP_INVALID` | That code is incorrect. Please try again. |
| `OUT_OF_CREDITS` | You’re out of bid credits. |
| `PACKAGE_NOT_FOUND` | That bid package is unavailable. |
| `PRICE_CHANGED` | The price has moved. Review the new price and try again. |
| `RATE_LIMITED` | Too many requests. Please slow down. |
| `RESERVATION_REQUIRED` | Lock your 10% deposit to reserve a place. |
| `RESOURCE_EXISTS` | This already exists. |
| `RESOURCE_INVALID` | That action isn’t allowed right now. |
| `RESOURCE_NOT_FOUND` | Not found. |
| `TIER_TOO_LOW` | An invitation is required to participate. |
| `UNAUTHENTICATED` | Please sign in to continue. |
| `VALIDATION_FAILED` | Please check the highlighted fields and try again. |
| `WEEKLY_CAP_REACHED` | This week’s catalogue is full. Try again next week. |

