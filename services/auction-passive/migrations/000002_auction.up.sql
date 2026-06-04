-- auction: a timed passive lot (Vickrey or UniqBid). This service owns the
-- passive auction state machine (CLAUDE.md §3): DRAFT -> APPRAISING -> SCHEDULED
-- -> OPEN -> CLOSING -> RESOLVED -> SETTLING -> COMPLETED, with ABORTED for a
-- UniqBid that resolves to no unique price. Money columns are BIGINT USDC cents
-- (CLAUDE.md §0, §7). The auction is created from catalog's lot.scheduled event.
CREATE TABLE auction (
    id                  UUID PRIMARY KEY,
    lot_id              UUID        NOT NULL,
    atype               TEXT        NOT NULL
                            CHECK (atype IN ('VICKREY', 'UNIQBID')),
    state               TEXT        NOT NULL DEFAULT 'SCHEDULED'
                            CHECK (state IN (
                                'DRAFT', 'APPRAISING', 'SCHEDULED', 'OPEN',
                                'CLOSING', 'RESOLVED', 'SETTLING', 'COMPLETED', 'ABORTED')),
    closes_at           TIMESTAMPTZ NOT NULL,
    reserve_cents       BIGINT      NOT NULL DEFAULT 0,
    winner_account_id   UUID        NULL,
    cleared_price_cents BIGINT      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One auction per source lot (idempotent lot.scheduled consumption).
CREATE UNIQUE INDEX uq_auction_lot_id ON auction (lot_id);

-- "active window, status=OPEN" read pattern: the close sweep + standing reads.
CREATE INDEX idx_auction_state_closes ON auction (state, closes_at);
