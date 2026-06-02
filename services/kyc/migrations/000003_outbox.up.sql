-- outbox: transactional outbox. A row is written in the same tx as the decision
-- it describes (kyc.approved / kyc.rejected); a background relay publishes each
-- to NATS and stamps published_at. idempotency_key is unique so re-decisions and
-- re-publishes dedup downstream.
CREATE TABLE outbox (
    id              UUID PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    subject         TEXT NOT NULL,
    payload         BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at    TIMESTAMPTZ NULL
);

-- Relay scans for unpublished rows oldest-first.
CREATE INDEX idx_outbox_unpublished ON outbox (created_at) WHERE published_at IS NULL;
