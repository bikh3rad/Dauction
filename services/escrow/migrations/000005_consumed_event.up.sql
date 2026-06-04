-- consumed_event: the inbox / idempotency ledger. A consumed event's
-- (subject-scoped) idempotency_key is recorded here so replays and out-of-order
-- redeliveries of auction.hammer / auction.won / dispute.resolved /
-- escrow.lock_requested are no-ops (root CLAUDE.md §0, §2).
CREATE TABLE consumed_event (
    idempotency_key TEXT PRIMARY KEY,
    consumed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
