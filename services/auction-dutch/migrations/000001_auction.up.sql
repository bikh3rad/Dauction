-- auction: a live descending-price (Dutch) auction (root CLAUDE.md §3). Money
-- columns are BIGINT USDC cents (§0, §7). The state machine
-- DRAFT -> APPRAISING -> SCHEDULED -> OPEN -> HAMMER -> SETTLING -> COMPLETED
-- (CANCELLED / ABORTED terminal) is CHECK-constrained. open_at is the price
-- clock origin: current_price(now) = max(floor, ceiling − step·⌊(now−open_at)/interval⌋).
CREATE TABLE auction (
    id                    UUID PRIMARY KEY,
    lot_id                UUID        NOT NULL, -- from catalog lot.scheduled
    state                 TEXT        NOT NULL DEFAULT 'SCHEDULED'
                              CHECK (state IN (
                                  'DRAFT', 'APPRAISING', 'SCHEDULED', 'OPEN',
                                  'HAMMER', 'SETTLING', 'COMPLETED', 'CANCELLED', 'ABORTED')),
    ceiling_cents         BIGINT      NOT NULL DEFAULT 0,
    floor_cents           BIGINT      NOT NULL DEFAULT 0,
    drop_step_cents       BIGINT      NOT NULL DEFAULT 0,
    drop_interval_seconds BIGINT      NOT NULL DEFAULT 0,
    open_at               TIMESTAMPTZ NULL, -- set at OPEN; clock origin for price(now)
    hammer_at             TIMESTAMPTZ NULL,
    winner_account_id     UUID        NULL,
    hammer_price_cents    BIGINT      NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- The descending curve must be well-formed (floor never above ceiling).
    CONSTRAINT auction_floor_le_ceiling CHECK (floor_cents <= ceiling_cents)
);

-- One auction per scheduled lot.
CREATE UNIQUE INDEX uq_auction_lot ON auction (lot_id);

-- "active, OPEN" read pattern (root CLAUDE.md §7).
CREATE INDEX idx_auction_state ON auction (state);
