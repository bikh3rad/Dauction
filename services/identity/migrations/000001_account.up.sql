-- account: the identity-owned user record. Tier and a mirrored KYC status power
-- the gateway's participation guard. Tier only ever rises (enforced in biz).
CREATE TABLE account (
    id         UUID PRIMARY KEY,
    tier       TEXT        NOT NULL DEFAULT 'GUEST'
                   CHECK (tier IN ('GUEST', 'MEMBER', 'VIP')),
    kyc_status TEXT        NOT NULL DEFAULT 'PENDING'
                   CHECK (kyc_status IN ('PENDING', 'APPROVED', 'REJECTED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Read pattern for the gateway guard / eligibility filters.
CREATE INDEX idx_account_tier_kyc ON account (tier, kyc_status);
