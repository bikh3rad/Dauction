-- passive_bid: the IMMUTABLE, append-only bid log (CLAUDE.md §3, §7). Rows are
-- never updated or deleted; resolution (Vickrey 2nd-price / UniqBid lowest-unique)
-- is a pure function of this log plus placed_at (the server clock tiebreaker).
-- price_cents is BIGINT USDC cents. Every accepted bid carries a confirmed
-- bids.Debit recorded by debit_idempotency_key, so a credit is never burned
-- without a recorded bid (CLAUDE.md §5).
CREATE TABLE passive_bid (
    id                    UUID PRIMARY KEY,
    auction_id            UUID        NOT NULL REFERENCES auction (id),
    bidder_account_id     UUID        NOT NULL,
    price_cents           BIGINT      NOT NULL CHECK (price_cents > 0),
    placed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    debit_idempotency_key TEXT        NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Resolution scan: all bids of an auction ordered by the tiebreaker clock.
CREATE INDEX idx_passive_bid_auction_placed ON passive_bid (auction_id, placed_at);

-- VICKREY: exactly one sealed bid per (auction, bidder).
-- UNIQBID: a bidder may submit many DISTINCT prices, but never the same price twice.
-- A single partial unique index over (auction, bidder, price) enforces UniqBid's
-- distinct-price rule; the per-mode "one bid per bidder" rule for VICKREY is
-- enforced in biz (the engine differs by atype) and additionally guarded by this
-- index (a Vickrey re-bid at a different price is rejected in biz before insert).
CREATE UNIQUE INDEX uq_passive_bid_auction_bidder_price
    ON passive_bid (auction_id, bidder_account_id, price_cents);

-- The debit key is globally unique so a replayed debit can never back two bids.
CREATE UNIQUE INDEX uq_passive_bid_debit_key ON passive_bid (debit_idempotency_key);
