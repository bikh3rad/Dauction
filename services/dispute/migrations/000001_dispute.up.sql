-- dispute: the dispute-court record for a trade. A buyer (claimant) raises a
-- post-delivery claim against the seller (respondent); the house rules; escrow
-- executes the verdict. State machine OPEN -> UNDER_REVIEW -> RESOLVED, with
-- WITHDRAWN as a terminal off-ramp (CLAUDE.md §4). All enums CHECK-constrained.
CREATE TABLE dispute (
    id                    UUID PRIMARY KEY,
    trade_id              TEXT        NOT NULL,
    claimant_account_id   UUID        NOT NULL,
    respondent_account_id UUID        NOT NULL,
    reason_code           TEXT        NOT NULL
                              CHECK (reason_code IN ('AUTHENTICITY', 'CONDITION', 'NOT_DELIVERED', 'OTHER')),
    state                 TEXT        NOT NULL DEFAULT 'OPEN'
                              CHECK (state IN ('OPEN', 'UNDER_REVIEW', 'RESOLVED', 'WITHDRAWN')),
    -- ruling is NULL until RESOLVED, then immutable (REFUND_BUYER/RELEASE_SELLER/SPLIT).
    ruling                TEXT        NULL
                              CHECK (ruling IS NULL OR ruling IN ('REFUND_BUYER', 'RELEASE_SELLER', 'SPLIT')),
    evidence_ref          TEXT        NOT NULL DEFAULT '',
    ruled_by              UUID        NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at           TIMESTAMPTZ NULL,
    -- claimant and respondent must differ; a RESOLVED dispute must carry a ruling.
    CONSTRAINT dispute_parties_differ CHECK (claimant_account_id <> respondent_account_id),
    CONSTRAINT dispute_resolved_has_ruling CHECK (state <> 'RESOLVED' OR ruling IS NOT NULL)
);

-- Only one NON-TERMINAL dispute may exist per trade (CLAUDE.md state rules): a
-- second open while one is OPEN/UNDER_REVIEW violates this and surfaces as
-- ErrResourceExists. Terminal disputes (RESOLVED/WITHDRAWN) are excluded so a
-- trade can be re-disputed after a withdrawal.
CREATE UNIQUE INDEX uq_dispute_open_per_trade
    ON dispute (trade_id)
    WHERE state IN ('OPEN', 'UNDER_REVIEW');

-- Admin queue read pattern: filter by state, newest first.
CREATE INDEX idx_dispute_state ON dispute (state, created_at DESC);

-- Lookup the current dispute for a trade.
CREATE INDEX idx_dispute_trade ON dispute (trade_id, created_at DESC);
