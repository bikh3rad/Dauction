-- Auth refactor (invites removed) + RBAC. Identity now owns mobile-number + OTP
-- auth, social OAuth, functional roles, and account status. MEMBER tier is driven
-- by kyc.approved (see internal/biz/consumer.go); these tables add the auth surface.

-- account gains: international mobile (E.164), mobile-verification timestamp, and
-- a lifecycle status orthogonal to tier/role.
ALTER TABLE account
    ADD COLUMN IF NOT EXISTS mobile_e164         TEXT,
    ADD COLUMN IF NOT EXISTS mobile_verified_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS handle              TEXT,
    ADD COLUMN IF NOT EXISTS status              TEXT NOT NULL DEFAULT 'REGISTERED'
        CHECK (status IN ('REGISTERED', 'ACTIVE', 'SUSPENDED', 'BANNED'));

-- One account per mobile number (partial unique: nulls allowed for OAuth-only).
CREATE UNIQUE INDEX IF NOT EXISTS ux_account_mobile
    ON account (mobile_e164) WHERE mobile_e164 IS NOT NULL;

-- mobile_otp: short-lived, single-use codes. The raw code is never stored — only
-- a hash. Rate-limiting and attempt caps are enforced in biz.
CREATE TABLE IF NOT EXISTS mobile_otp (
    id          BIGSERIAL   PRIMARY KEY,
    mobile_e164 TEXT        NOT NULL,
    code_hash   TEXT        NOT NULL,
    purpose     TEXT        NOT NULL CHECK (purpose IN ('SIGNUP', 'LOGIN')),
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    attempts    INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_otp_mobile_active
    ON mobile_otp (mobile_e164) WHERE consumed_at IS NULL;

-- oauth_identity: one row per linked social provider identity.
CREATE TABLE IF NOT EXISTS oauth_identity (
    id               BIGSERIAL   PRIMARY KEY,
    account_id       UUID        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL CHECK (provider IN ('GOOGLE', 'FACEBOOK', 'APPLE')),
    provider_user_id TEXT        NOT NULL,
    email            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX IF NOT EXISTS idx_oauth_account ON oauth_identity (account_id);

-- account_role: RBAC. USER is implicit (no row); elevated roles get a row.
CREATE TABLE IF NOT EXISTS account_role (
    account_id UUID        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    role       TEXT        NOT NULL CHECK (role IN ('INSPECTOR', 'ADMIN')),
    granted_by UUID,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, role)
);
