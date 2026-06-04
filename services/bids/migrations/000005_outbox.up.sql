-- outbox: transactional outbox. A state change (a purchase grant or a debit) and
-- its event row commit in ONE transaction; a background publisher relays
-- unpublished rows to NATS/JetStream. idempotency_key is producer-stable so
-- consumers dedup (CLAUDE.md §0, §5). For a debit, idempotency_key matches the
-- bid_debit row so the emission is exactly-once per logical debit.
CREATE TABLE outbox (
    id              UUID PRIMARY KEY,
    subject         TEXT        NOT NULL,
    idempotency_key TEXT        NOT NULL UNIQUE,
    payload         JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at    TIMESTAMPTZ NULL
);

-- Relay scan: oldest unpublished first.
CREATE INDEX idx_outbox_unpublished ON outbox (created_at) WHERE published_at IS NULL;
