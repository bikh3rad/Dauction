-- Categories, the Inspector verification gate, lot media carousel, and 4-language
-- lot content (CLAUDE.md §3.5, §7). The lot state machine is unchanged: DRAFT =
-- awaiting inspection; an APPROVED inspection drives DRAFT -> CERTIFIED; a FAIL
-- drives -> REJECTED. Additive and backward-compatible.

-- Category catalog. Codes are language-neutral; the client localizes display
-- names. icon_key maps to the design-team per-category icon set.
CREATE TABLE IF NOT EXISTS category (
    id         BIGSERIAL PRIMARY KEY,
    code       TEXT    NOT NULL UNIQUE,
    icon_key   TEXT    NOT NULL,
    sort_order INT     NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO category (code, icon_key, sort_order) VALUES
    ('WATCHES',      'cat-watches',      10),
    ('JEWELRY',      'cat-jewelry',      20),
    ('FINE_ART',     'cat-fine-art',     30),
    ('AUTOMOBILES',  'cat-automobiles',  40),
    ('HANDBAGS',     'cat-handbags',     50),
    ('RARE_SPIRITS', 'cat-rare-spirits', 60),
    ('COLLECTIBLES', 'cat-collectibles', 70)
ON CONFLICT (code) DO NOTHING;

ALTER TABLE lot
    ADD COLUMN IF NOT EXISTS category_code       TEXT,
    ADD COLUMN IF NOT EXISTS certified           BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS inspector_account_id UUID,
    ADD COLUMN IF NOT EXISTS authenticity        TEXT
        CHECK (authenticity IS NULL OR authenticity IN ('GENUINE', 'COUNTERFEIT', 'INCONCLUSIVE')),
    ADD COLUMN IF NOT EXISTS condition_grade     TEXT
        CHECK (condition_grade IS NULL OR condition_grade IN ('MINT', 'EXCELLENT', 'GOOD', 'FAIR', 'POOR'));

-- The Inspector's sealing verdict — the certification gate. One verdict per lot.
CREATE TABLE IF NOT EXISTS inspection (
    id              UUID PRIMARY KEY,
    lot_id          UUID        NOT NULL UNIQUE REFERENCES lot (id) ON DELETE CASCADE,
    inspector_id    UUID        NOT NULL,
    verdict         TEXT        NOT NULL CHECK (verdict IN ('APPROVED', 'REJECTED')),
    authenticity    TEXT        NOT NULL CHECK (authenticity IN ('GENUINE', 'COUNTERFEIT', 'INCONCLUSIVE')),
    condition_grade TEXT        CHECK (condition_grade IS NULL OR condition_grade IN ('MINT', 'EXCELLENT', 'GOOD', 'FAIR', 'POOR')),
    notes           TEXT        NOT NULL DEFAULT '',
    sealed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Inspector queue read: lots in DRAFT (awaiting a seal), oldest first.
CREATE INDEX IF NOT EXISTS idx_lot_inspection_queue ON lot (state, created_at)
    WHERE state = 'DRAFT';

-- 4-language lot content, mirrored from the object.listed payload (catalog never
-- reads vault's DB). Returned WHOLE; the client picks the active language.
CREATE TABLE IF NOT EXISTS lot_translation (
    lot_id      UUID NOT NULL REFERENCES lot (id) ON DELETE CASCADE,
    lang        TEXT NOT NULL CHECK (lang IN ('en', 'fa', 'ar', 'tr')),
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    PRIMARY KEY (lot_id, lang)
);

-- ≤7 carousel images on the public lot (mirrored from the listing). DB invariant.
CREATE TABLE IF NOT EXISTS lot_media (
    id          BIGSERIAL   PRIMARY KEY,
    lot_id      UUID        NOT NULL REFERENCES lot (id) ON DELETE CASCADE,
    position    SMALLINT    NOT NULL CHECK (position BETWEEN 0 AND 6),
    storage_key TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (lot_id, position)
);
