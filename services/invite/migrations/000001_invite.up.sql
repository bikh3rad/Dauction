-- invite service schema (CLAUDE.md §7). Money-free; account ids are opaque
-- strings owned by the identity service (no FK across service DBs).

-- Single-use invitation codes. State machine (CLAUDE.md):
--   ISSUED -> REDEEMED (terminal) | ISSUED -> REVOKED | ISSUED -> FLAGGED
CREATE TABLE invite (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code              TEXT NOT NULL,
    issuer_account_id TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'ISSUED'
                          CHECK (status IN ('ISSUED', 'REDEEMED', 'REVOKED', 'FLAGGED')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at       TIMESTAMPTZ NULL,
    redeemed_by       TEXT NULL,
    -- a redeemed code must carry both redemption columns; non-redeemed must not.
    CONSTRAINT invite_redemption_consistent CHECK (
        (status = 'REDEEMED' AND redeemed_at IS NOT NULL AND redeemed_by IS NOT NULL)
        OR (status <> 'REDEEMED' AND redeemed_at IS NULL AND redeemed_by IS NULL)
    )
);

-- A code is globally unique and the single-use redemption keys off it.
CREATE UNIQUE INDEX idx_invite_code ON invite (code);
-- Admin listing by issuer / status.
CREATE INDEX idx_invite_issuer ON invite (issuer_account_id);
CREATE INDEX idx_invite_status ON invite (status);

-- Invite chain: one edge recorded per redemption (inviter -> invitee).
CREATE TABLE invite_edge (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code               TEXT NOT NULL,
    inviter_account_id TEXT NOT NULL,
    invitee_account_id TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Chain traversal by inviter (who an account brought in) and by invitee.
CREATE INDEX idx_invite_edge_inviter ON invite_edge (inviter_account_id);
CREATE INDEX idx_invite_edge_invitee ON invite_edge (invitee_account_id);
-- One redemption per code -> one edge per code.
CREATE UNIQUE INDEX idx_invite_edge_code ON invite_edge (code);

-- Transactional outbox: rows are written in the same tx as the domain change
-- (CLAUDE.md §0/§2). The background publisher relays unpublished rows to NATS
-- JetStream; idempotency_key lets consumers dedup (frozen EventEnvelope shape).
CREATE TABLE outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    producer        TEXT NOT NULL,
    type            TEXT NOT NULL,
    version         INTEGER NOT NULL DEFAULT 1,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload         JSONB NOT NULL,
    published_at    TIMESTAMPTZ NULL
);

-- Producer-stable dedup key: a logical write is enqueued at most once.
CREATE UNIQUE INDEX idx_outbox_idempotency_key ON outbox (idempotency_key);
-- Publisher poll scan over the un-relayed tail.
CREATE INDEX idx_outbox_unpublished ON outbox (occurred_at) WHERE published_at IS NULL;
