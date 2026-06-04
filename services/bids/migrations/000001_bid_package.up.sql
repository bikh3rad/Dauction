-- bid_package: the purchasable credit-bundle catalogue (CLAUDE.md §5). Each credit
-- is $1; `credits` is WHOLE bid credits, `price_cents` is USDC cents — DISTINCT
-- units, never mixed. Read-only at runtime; seeded here.
CREATE TABLE bid_package (
    id          TEXT PRIMARY KEY,
    credits     BIGINT      NOT NULL CHECK (credits > 0),
    price_cents BIGINT      NOT NULL CHECK (price_cents > 0),
    best_value  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed packages: 100 -> $80, 50 -> $45, 20 -> $20.
INSERT INTO bid_package (id, credits, price_cents, best_value) VALUES
    ('PKG_100', 100, 8000, TRUE),
    ('PKG_50',   50, 4500, FALSE),
    ('PKG_20',   20, 2000, FALSE);
