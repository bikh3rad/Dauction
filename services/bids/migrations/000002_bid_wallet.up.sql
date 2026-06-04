-- bid_wallet: the read-through bid-credit balance per account (CLAUDE.md §5).
-- balance_credits is WHOLE bid credits ($1 each), NOT USDC cents. The CHECK
-- guarantees the balance can never go negative; the debit path uses a conditional
-- UPDATE (`... WHERE balance_credits >= $n`) so an over-debit simply matches no row.
CREATE TABLE bid_wallet (
    account_id      UUID PRIMARY KEY,
    balance_credits BIGINT      NOT NULL DEFAULT 0 CHECK (balance_credits >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
