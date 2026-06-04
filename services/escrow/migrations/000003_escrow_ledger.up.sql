-- escrow_ledger: the APPEND-ONLY funds ledger (root CLAUDE.md §4). NEVER updated
-- or deleted. `escrow` is the sole writer. amount_cents is SIGNED BIGINT USDC
-- cents: locks/holds/release/forfeit/fee/premium/inspector_fee are positive;
-- refunds are negative. The derived per-(trade, participant) balance is
-- SUM(amount_cents). The funds-conservation invariant holds: once locked, the
-- gross inflows {DEPOSIT_LOCK, FULL_LOCK, HOLD} are constant and the gross
-- disbursements {RELEASE, REFUND, FORFEIT, FEE, PREMIUM, INSPECTOR_FEE} settle
-- to exactly that amount.
CREATE TABLE escrow_ledger (
    id                     UUID PRIMARY KEY,
    trade_id               UUID        NOT NULL REFERENCES escrow_trade (id),
    participant_account_id UUID        NOT NULL,
    entry_type             TEXT        NOT NULL
                               CHECK (entry_type IN (
                                   'DEPOSIT_LOCK', 'FULL_LOCK', 'HOLD', 'RELEASE',
                                   'REFUND', 'FORFEIT', 'FEE', 'PREMIUM', 'INSPECTOR_FEE')),
    amount_cents           BIGINT      NOT NULL,                -- signed USDC cents
    ref                    TEXT        NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Resolution / balance derivation: per-trade rows oldest-first.
CREATE INDEX idx_escrow_ledger_trade ON escrow_ledger (trade_id, created_at);
-- Per-(trade, participant) balance derivation.
CREATE INDEX idx_escrow_ledger_trade_participant ON escrow_ledger (trade_id, participant_account_id);
