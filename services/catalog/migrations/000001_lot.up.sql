-- lot: a catalog-owned gallery lot derived from a vault object.listed event.
-- Money columns are BIGINT USDC cents (CLAUDE.md §0, §7). The catalog state
-- machine (DRAFT -> CERTIFIED -> SCHEDULED, REJECTED terminal) is CHECK-constrained;
-- the weekly 32-cap is enforced in biz/repo at schedule time.
CREATE TABLE lot (
    id                    UUID PRIMARY KEY,
    object_id             UUID        NOT NULL,
    seller_account_id     UUID        NOT NULL,
    title                 TEXT        NOT NULL DEFAULT '',
    description           TEXT        NOT NULL DEFAULT '',
    atype                 TEXT        NOT NULL
                              CHECK (atype IN ('DUTCH', 'VICKREY', 'UNIQBID')),
    duration_days         INTEGER     NULL
                              CHECK (duration_days IS NULL OR duration_days IN (2, 5, 7)),
    reserve_cents         BIGINT      NOT NULL DEFAULT 0,
    appraised_value_cents BIGINT      NOT NULL DEFAULT 0,
    state                 TEXT        NOT NULL DEFAULT 'DRAFT'
                              CHECK (state IN ('DRAFT', 'CERTIFIED', 'SCHEDULED', 'REJECTED')),
    iso_week              TEXT        NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    scheduled_at          TIMESTAMPTZ NULL,

    -- DUTCH is live and carries no timed duration; VICKREY/UNIQBID must.
    CONSTRAINT lot_duration_per_mode CHECK (
        (atype = 'DUTCH' AND duration_days IS NULL)
        OR (atype IN ('VICKREY', 'UNIQBID') AND duration_days IS NOT NULL)
    )
);

-- One lot per source vault object (idempotent object.listed consumption).
CREATE UNIQUE INDEX uq_lot_object_id ON lot (object_id);

-- "active week, state" read pattern: weekly gallery + the 32-cap count.
CREATE INDEX idx_lot_week_state ON lot (iso_week, state);
