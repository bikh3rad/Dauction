-- consumed_event: the inbox / idempotency ledger. dispute is currently
-- publish-only (it consumes no events), but the table is provisioned so a future
-- consumer (e.g. escrow.released auto-close) can dedup replays/out-of-order
-- redeliveries by recording the (subject-scoped) idempotency_key in the SAME
-- transaction as the state change it drives (CLAUDE.md §0, §2).
CREATE TABLE consumed_event (
    idempotency_key TEXT PRIMARY KEY,
    consumed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
