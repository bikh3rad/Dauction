-- Categories, ≤7 media, 4-language item content, and the inspection-aware listing
-- lifecycle (CLAUDE.md §3.5, §7). Additive: existing vault_object inserts keep
-- working; new columns are nullable / defaulted.

-- One Vault per account, auto-provisioned on account.registered.
CREATE TABLE IF NOT EXISTS vault (
    id         UUID PRIMARY KEY,
    account_id UUID        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE vault_object
    ADD COLUMN IF NOT EXISTS vault_id      UUID,
    ADD COLUMN IF NOT EXISTS category_code TEXT,
    ADD COLUMN IF NOT EXISTS primary_lang  TEXT
        CHECK (primary_lang IS NULL OR primary_lang IN ('en', 'fa', 'ar', 'tr'));

-- Widen the listing lifecycle to pass through inspection before going public:
-- IN_VAULT -> PENDING_INSPECTION -> APPROVED | REJECTED -> IN_AUCTION -> SOLD.
ALTER TABLE vault_object DROP CONSTRAINT IF EXISTS vault_object_state_check;
ALTER TABLE vault_object ADD CONSTRAINT vault_object_state_check CHECK (
    state IN ('IN_VAULT', 'PENDING_INSPECTION', 'APPROVED', 'REJECTED',
              'APPRAISING', 'IN_AUCTION', 'SOLD', 'BOUGHT_BACK')
);

-- 4-language title/description. All four rows are written (blanks back-filled from
-- the primary language in biz), so any gallery locale renders.
CREATE TABLE IF NOT EXISTS vault_object_translation (
    object_id   UUID NOT NULL REFERENCES vault_object (id) ON DELETE CASCADE,
    lang        TEXT NOT NULL CHECK (lang IN ('en', 'fa', 'ar', 'tr')),
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    PRIMARY KEY (object_id, lang)
);

-- Max 7 images per item — a DB invariant (position 0..6 + unique position), not
-- just an app check. position 0 is the cover.
CREATE TABLE IF NOT EXISTS vault_object_media (
    id           BIGSERIAL   PRIMARY KEY,
    object_id    UUID        NOT NULL REFERENCES vault_object (id) ON DELETE CASCADE,
    position     SMALLINT    NOT NULL CHECK (position BETWEEN 0 AND 6),
    storage_key  TEXT        NOT NULL,
    content_type TEXT        NOT NULL DEFAULT 'image/jpeg',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (object_id, position)
);
