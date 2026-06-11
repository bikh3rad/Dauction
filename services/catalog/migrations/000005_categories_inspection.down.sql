DROP TABLE IF EXISTS lot_media;
DROP TABLE IF EXISTS lot_translation;
DROP INDEX IF EXISTS idx_lot_inspection_queue;
DROP TABLE IF EXISTS inspection;
ALTER TABLE lot
    DROP COLUMN IF EXISTS condition_grade,
    DROP COLUMN IF EXISTS authenticity,
    DROP COLUMN IF EXISTS inspector_account_id,
    DROP COLUMN IF EXISTS certified,
    DROP COLUMN IF EXISTS category_code;
DROP TABLE IF EXISTS category;
