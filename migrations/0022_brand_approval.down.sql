DROP INDEX IF EXISTS inventory.idx_brand_status;
ALTER TABLE inventory.brand
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS submitted_by_user_id,
    DROP COLUMN IF EXISTS approved_by_user_id,
    DROP COLUMN IF EXISTS approved_at;
