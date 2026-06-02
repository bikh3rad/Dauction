-- kyc_submission: one identity-verification attempt per row. We store only a
-- document reference (external handle), never the document blob.
CREATE TABLE kyc_submission (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id       UUID NOT NULL,
    doc_type         TEXT NOT NULL CHECK (doc_type IN ('EMIRATES_ID', 'PASSPORT')),
    doc_ref          TEXT NOT NULL,
    phone            TEXT NOT NULL,
    state            TEXT NOT NULL DEFAULT 'STARTED'
                       CHECK (state IN ('STARTED', 'OTP_VERIFIED', 'SUBMITTED', 'APPROVED', 'REJECTED')),
    rejection_reason TEXT NULL,
    decided_by       UUID NULL,
    submitted_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at       TIMESTAMPTZ NULL
);

-- Status lookups are by account, newest first.
CREATE INDEX idx_kyc_submission_account ON kyc_submission (account_id, submitted_at DESC);
-- Admin queue scans submissions awaiting a decision.
CREATE INDEX idx_kyc_submission_state ON kyc_submission (state, submitted_at);
