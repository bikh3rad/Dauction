-- dispute_event: the IMMUTABLE audit trail (CLAUDE.md §4 "keep an immutable
-- audit trail"). Every dispute action appends exactly one row; rows are never
-- updated or deleted. action is CHECK-constrained to the lifecycle vocabulary.
CREATE TABLE dispute_event (
    id               UUID PRIMARY KEY,
    dispute_id       UUID        NOT NULL REFERENCES dispute (id),
    actor_account_id UUID        NOT NULL,
    action           TEXT        NOT NULL
                         CHECK (action IN ('OPENED', 'EVIDENCE_ADDED', 'REVIEW_STARTED', 'RULED', 'WITHDRAWN')),
    detail_ref       TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit-trail read: oldest-first per dispute.
CREATE INDEX idx_dispute_event_dispute ON dispute_event (dispute_id, created_at, id);
