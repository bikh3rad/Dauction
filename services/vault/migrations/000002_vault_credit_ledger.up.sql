-- vault_credit_ledger: append-only Vault-Credit ledger. An account's balance is
-- SUM(delta_cents). delta_cents is a SIGNED int64 USDC cents amount (BIGINT) of
-- Vault Credit — denominated in USDC, NOT bid credits (CLAUDE.md §5: never mix
-- the two units). reason is a machine code; ref_id ties the row to its source.
CREATE TABLE vault_credit_ledger (
    id         UUID PRIMARY KEY,
    account_id UUID        NOT NULL,
    delta_cents BIGINT     NOT NULL,
    reason     TEXT        NOT NULL
                   CHECK (reason IN ('BUYBACK', 'AUCTION_RELEASE', 'ADJUSTMENT')),
    ref_id     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Balance scan per account.
CREATE INDEX idx_vault_credit_account ON vault_credit_ledger (account_id);
