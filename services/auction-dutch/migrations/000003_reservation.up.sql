-- reservation: one escrow hold a participant takes on an auction — the 10%
-- reservation deposit (DEPOSIT_10) or the 100% full lock (FULL_LOCK), per the
-- Dutch escrow path (root CLAUDE.md §4). The escrow service is the funds
-- authority; this row mirrors the local request/lock state, keyed to escrow by
-- escrow_ref (the producer-stable idempotency key shared with escrow.lock_requested
-- and echoed back on escrow.locked). amount_cents is BIGINT USDC cents.
CREATE TABLE reservation (
    id           UUID PRIMARY KEY,
    auction_id   UUID        NOT NULL REFERENCES auction (id) ON DELETE CASCADE,
    account_id   UUID        NOT NULL,
    kind         TEXT        NOT NULL
                     CHECK (kind IN ('DEPOSIT_10', 'FULL_LOCK')),
    amount_cents BIGINT      NOT NULL DEFAULT 0,
    state        TEXT        NOT NULL DEFAULT 'REQUESTED'
                     CHECK (state IN ('REQUESTED', 'LOCKED', 'RELEASED')),
    escrow_ref   TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- escrow_ref is the idempotency key: one reservation per (auction, account, kind).
-- It dedups retried reserve/lock requests and lets escrow.locked find its row.
CREATE UNIQUE INDEX uq_reservation_escrow_ref ON reservation (escrow_ref);

CREATE INDEX idx_reservation_auction ON reservation (auction_id, account_id);
