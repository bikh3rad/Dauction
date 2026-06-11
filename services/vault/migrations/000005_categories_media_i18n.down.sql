DROP TABLE IF EXISTS vault_object_media;
DROP TABLE IF EXISTS vault_object_translation;
ALTER TABLE vault_object DROP CONSTRAINT IF EXISTS vault_object_state_check;
ALTER TABLE vault_object ADD CONSTRAINT vault_object_state_check CHECK (
    state IN ('IN_VAULT', 'APPRAISING', 'IN_AUCTION', 'SOLD', 'BOUGHT_BACK')
);
ALTER TABLE vault_object
    DROP COLUMN IF EXISTS primary_lang,
    DROP COLUMN IF EXISTS category_code,
    DROP COLUMN IF EXISTS vault_id;
DROP TABLE IF EXISTS vault;
