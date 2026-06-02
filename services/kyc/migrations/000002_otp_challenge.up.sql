-- otp_challenge: a single OTP issued against a submission's phone. Only the hash
-- of the code is stored; the plaintext code is never persisted.
CREATE TABLE otp_challenge (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES kyc_submission (id) ON DELETE CASCADE,
    phone         TEXT NOT NULL,
    code_hash     TEXT NOT NULL,
    attempts      INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Verify path fetches the open (unverified) challenge for a submission.
CREATE INDEX idx_otp_challenge_submission ON otp_challenge (submission_id, verified, created_at DESC);
