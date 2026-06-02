-- vault_object: a single item in a member's private collection. State follows
-- IN_VAULT -> APPRAISING -> IN_AUCTION -> SOLD, with BOUGHT_BACK as a terminal
-- buyback state (CLAUDE.md §1, §7). appraised_value_cents is int64 USDC cents
-- (BIGINT) — money is never a float.
CREATE TABLE vault_object (
    id                    UUID PRIMARY KEY,
    owner_account_id      UUID        NOT NULL,
    title                 TEXT        NOT NULL,
    description           TEXT        NOT NULL DEFAULT '',
    appraised_value_cents BIGINT      NOT NULL CHECK (appraised_value_cents > 0),
    state                 TEXT        NOT NULL DEFAULT 'IN_VAULT'
                              CHECK (state IN ('IN_VAULT', 'APPRAISING', 'IN_AUCTION', 'SOLD', 'BOUGHT_BACK')),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Owner's vault read (GET /apis/vault), newest first.
CREATE INDEX idx_vault_object_owner ON vault_object (owner_account_id, created_at DESC);
