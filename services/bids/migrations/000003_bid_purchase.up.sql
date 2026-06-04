-- bid_purchase: one row per credit-package purchase (CLAUDE.md §5). The USDC charge
-- (usdc_charged_cents) and the credit grant (credits_granted) are recorded together
-- and committed atomically with the wallet credit + the bids.purchased outbox row.
-- credits_granted is WHOLE bid credits; usdc_charged_cents is USDC cents — DISTINCT
-- units in DISTINCT columns, never mixed.
CREATE TABLE bid_purchase (
    id                 UUID PRIMARY KEY,
    account_id         UUID        NOT NULL,
    package_id         TEXT        NOT NULL REFERENCES bid_package (id),
    credits_granted    BIGINT      NOT NULL CHECK (credits_granted > 0),
    usdc_charged_cents BIGINT      NOT NULL CHECK (usdc_charged_cents >= 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Read pattern: a member's recent purchases for the wallet view.
CREATE INDEX idx_bid_purchase_account ON bid_purchase (account_id, created_at DESC);
