-- bid_debit: the immutable, idempotent debit-on-bid log (CLAUDE.md §5). One row per
-- logical debit. idempotency_key is UNIQUE: a replay with the same key matches the
-- existing row (ON CONFLICT DO NOTHING) and burns NOTHING — the original debit is
-- returned. The row insert + the conditional wallet UPDATE + the bids.debited outbox
-- row all commit in ONE transaction so a credit is never burned without a recorded
-- debit (and vice-versa). amount_credits is WHOLE bid credits.
CREATE TABLE bid_debit (
    id              UUID PRIMARY KEY,
    account_id      UUID        NOT NULL,
    amount_credits  BIGINT      NOT NULL CHECK (amount_credits > 0),
    idempotency_key TEXT        NOT NULL UNIQUE,
    auction_id      TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Read pattern: a member's recent debits for the wallet view.
CREATE INDEX idx_bid_debit_account ON bid_debit (account_id, created_at DESC);
