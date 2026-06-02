-- attestation: the inspector's recorded seal on a lot (CLAUDE.md §1, §2, §7).
-- Append-only record backing the certification gate: a PASS unlocks CERTIFIED.
CREATE TABLE attestation (
    id           UUID PRIMARY KEY,
    lot_id       UUID        NOT NULL REFERENCES lot (id),
    inspector_id UUID        NOT NULL,
    result       TEXT        NOT NULL
                     CHECK (result IN ('PASS', 'FAIL')),
    notes_ref    TEXT        NOT NULL DEFAULT '',
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Certification gate lookup: attestations by lot (and the PASS existence check).
CREATE INDEX idx_attestation_lot ON attestation (lot_id, recorded_at);
