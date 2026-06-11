DROP TABLE IF EXISTS account_role;
DROP TABLE IF EXISTS oauth_identity;
DROP TABLE IF EXISTS mobile_otp;
DROP INDEX IF EXISTS ux_account_mobile;
ALTER TABLE account
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS handle,
    DROP COLUMN IF EXISTS mobile_verified_at,
    DROP COLUMN IF EXISTS mobile_e164;
