-- escrow_trade: the per-trade head record (trade_id == auction_id). Carries the
-- obligation (price + premium + fees) and the current funds-machine state. The
-- signed rows in escrow_ledger are the source of truth for balances; this row is
-- the derived state head plus immutable trade terms. `escrow` is the sole writer
-- of escrow rows (root CLAUDE.md §4). All *_cents columns are BIGINT USDC cents.
CREATE TABLE escrow_trade (
    id                  UUID PRIMARY KEY,                       -- trade_id == auction_id
    lot_id              UUID        NOT NULL,
    buyer_account_id    UUID        NOT NULL,                   -- winner / buyer obligor
    seller_account_id   UUID        NOT NULL,                   -- release beneficiary
    kind                TEXT        NOT NULL
                            CHECK (kind IN ('DUTCH', 'PASSIVE')),
    state               TEXT        NOT NULL
                            CHECK (state IN (
                                'UNLOCKED', 'DEPOSIT_LOCKED', 'FULL_LOCKED', 'HELD',
                                'RELEASED', 'REFUNDED', 'FORFEITED', 'DISPUTED')),
    price_cents         BIGINT      NOT NULL DEFAULT 0,         -- hammer / cleared price
    premium_cents       BIGINT      NOT NULL DEFAULT 0,         -- buyer's premium
    fee_cents           BIGINT      NOT NULL DEFAULT 0,         -- house fee
    inspector_fee_cents BIGINT      NOT NULL DEFAULT 0,         -- inspector attestation fee
    release_mode        TEXT        NULL
                            CHECK (release_mode IS NULL OR release_mode IN ('CASH', 'VAULT_CREDIT')),
    funding_deadline    TIMESTAMPTZ NULL,                       -- winner must fund by this instant (24h window)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Operational scans: funding-window sweeps over open obligations.
CREATE INDEX idx_escrow_trade_state ON escrow_trade (state);
CREATE INDEX idx_escrow_trade_funding_deadline ON escrow_trade (funding_deadline)
    WHERE funding_deadline IS NOT NULL;
