-- consumed_event: the inbox / idempotency ledger. A consumed event's
-- (subject-scoped) idempotency_key is recorded here, in the SAME transaction as
-- the state change it drives (here: the auction created from lot.scheduled), so
-- replays and out-of-order redeliveries are no-ops (CLAUDE.md §0, §2).
CREATE TABLE consumed_event (
    idempotency_key TEXT PRIMARY KEY,
    consumed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
